package loop

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const reviewBypassedOutputRef = "review:bypassed"

func evaluateActionReview(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	outputs []GenerationOutput,
) (GenerationOutput, error) {
	review := node.Review
	if review == nil {
		return output, nil
	}
	namespace, err := runtimeNamespaceWithHistory(
		eval.run, eval.generation, eval.resolved.Definition.Graph, eval.topology,
		outputs, eval.history, node.ID, output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, err
	}
	if strings.TrimSpace(review.When) != "" {
		key := fmt.Sprintf("nodes.%s.review.when", node.ID)
		condition := eval.resolved.Conditions[key]
		if condition == nil {
			return GenerationOutput{}, fmt.Errorf("%w: compiled review condition %q is missing", ErrValidation, key)
		}
		evaluated, evalErr := evaluatePredicate(key, condition, namespace, PredicateRouting, dsl.EvalErrorFail)
		if evalErr != nil {
			return GenerationOutput{}, fmt.Errorf("loop: evaluate review %s: %w", node.ID, evalErr)
		}
		eval.gateEvaluations.recordPredicate(evaluated.Diagnostics...)
		if evaluated.Disposition != nil {
			failed, failureErr := applyPredicateFailureDisposition(
				output, node, evaluated.Disposition.Failure, eval.resolved.Definition.Graph,
				eval.topology, &outputs,
			)
			return failed, failureErr
		}
		if !evaluated.Value {
			setGenerationOutputRef(&output, reviewBypassedOutputRef)
			return output, nil
		}
	}
	rendered, err := materializeReviewProposal(node, namespace)
	if err != nil {
		return GenerationOutput{}, fmt.Errorf("loop: render review proposal %q: %w", node.ID, err)
	}
	proposed, err := json.Marshal(rendered)
	if err != nil {
		return GenerationOutput{}, fmt.Errorf("loop: marshal review proposal %q: %w", node.ID, err)
	}
	_, proposedPreview, err := requestContextPayloads(rendered)
	if err != nil {
		return GenerationOutput{}, err
	}
	prompt := strings.TrimSpace(review.Prompt)
	if prompt == "" {
		prompt = "Review " + string(node.ID)
	} else {
		renderedPrompt, renderErr := renderActionParam("nodes."+string(node.ID)+".review.prompt", prompt, namespace)
		if renderErr != nil {
			return GenerationOutput{}, fmt.Errorf("loop: render review prompt %q: %w", node.ID, renderErr)
		}
		var ok bool
		prompt, ok = renderedPrompt.(string)
		if !ok || strings.TrimSpace(prompt) == "" {
			return GenerationOutput{}, fmt.Errorf("%w: review prompt for %q must resolve to a string", ErrValidation, node.ID)
		}
	}
	decisions := review.EffectiveDecisions()
	decisionStrings := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		decisionStrings = append(decisionStrings, string(decision))
	}
	decisionsRaw, err := json.Marshal(decisionStrings)
	if err != nil {
		return GenerationOutput{}, fmt.Errorf("loop: marshal review decisions %q: %w", node.ID, err)
	}
	editSchema, respondSchema, err := reviewSchemas(eval.resolved, node, rendered, decisions)
	if err != nil {
		return GenerationOutput{}, err
	}
	now := eval.now.UTC()
	expiresAt, err := reviewExpiresAt(review, eval.effective.RequestExpireAfter, now)
	if err != nil {
		return GenerationOutput{}, fmt.Errorf("loop: review node %q: %w", node.ID, err)
	}
	contextRaw := json.RawMessage(`{}`)
	output.Status = generationOutputWaiting
	output.TaskRunID = ""
	output.NextAttemptAt = nil
	output.Epoch++
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return GenerationOutput{}, err
	}
	payload.Waits = append(payload.Waits, NodeWaitIntent{
		NodeID: NodeID(output.NodeID), ItemIndex: output.ItemIndex, Kind: NodeWaitKindRequest,
		NextEscalationAt: expiresAt, IssuedEpoch: output.Epoch, CreatedAt: now,
	})
	agents := dsl.ResponderAgentsDeny
	if review.Responders != nil && review.Responders.Agents != "" {
		agents = review.Responders.Agents
	}
	payload.Requests = append(payload.Requests, RequestIntent{
		WorkspaceID: eval.run.WorkspaceID, NodeID: node.ID, ItemIndex: output.ItemIndex,
		Kind: RequestKindReview, Prompt: diagnostics.RedactAndBound(prompt, requestPreviewLimitBytes),
		Context: contextRaw, ContextPreview: contextRaw, EditSchema: editSchema,
		RespondSchema: respondSchema, Decisions: decisionsRaw, Proposed: proposed,
		ProposedPreview: proposedPreview, Agents: agents, ExpiresAt: expiresAt,
		IssuedEpoch: output.Epoch, OpenedAt: now,
	})
	payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
		Kind: GenerationLifecycleEventNodeWaitStarted, NodeID: output.NodeID,
		ItemIndex: output.ItemIndex, Attempt: output.Attempt, IssuedEpoch: output.Epoch,
		WaitKind: NodeWaitKindRequest, NextAttemptAt: expiresAt,
	})
	plan.Snapshot.Payload = payload
	return output, nil
}

func materializeReviewProposal(node dsl.Node, namespace map[string]any) (map[string]any, error) {
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionRunAgent:
		return renderNodeParamsExcept(node, namespace, map[string]struct{}{outputSchemaParamKey: {}})
	case dsl.ActionGoal:
		params, err := MaterializeGoalParams(node, namespace)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("loop: marshal reviewed Goal params: %w", err)
		}
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("loop: decode reviewed Goal params: %w", err)
		}
		return result, nil
	default:
		return renderNodeParams(node, namespace)
	}
}

func reviewSchemas(
	resolved *ResolvedDefinition,
	node dsl.Node,
	rendered map[string]any,
	decisions []dsl.ReviewDecision,
) (json.RawMessage, json.RawMessage, error) {
	var editSchema, respondSchema json.RawMessage
	for _, decision := range decisions {
		switch decision {
		case dsl.ReviewDecisionEdit:
			if !dsl.IsReservedActionKind(node.Kind) {
				snapshot, ok := resolved.ToolSchemas[node.Kind]
				if !ok || len(snapshot.InputSchema) == 0 {
					return nil, nil, fmt.Errorf("%w: review input schema for %q is missing", ErrValidation, node.ID)
				}
				editSchema = cloneRawMessage(snapshot.InputSchema)
			} else {
				schema := inferredReviewSchema(rendered)
				encoded, err := json.Marshal(schema)
				if err != nil {
					return nil, nil, fmt.Errorf("loop: marshal review edit schema %q: %w", node.ID, err)
				}
				editSchema = encoded
			}
		case dsl.ReviewDecisionRespond:
			var err error
			respondSchema, err = actionOutputSchemaRaw(resolved, node)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return editSchema, respondSchema, nil
}

func actionOutputSchemaRaw(resolved *ResolvedDefinition, node dsl.Node) (json.RawMessage, error) {
	if len(node.Produces) > 0 {
		return json.Marshal(node.Produces)
	}
	if !dsl.IsReservedActionKind(node.Kind) {
		snapshot, ok := resolved.ToolSchemas[node.Kind]
		if !ok || len(snapshot.OutputSchema) == 0 {
			return nil, fmt.Errorf("%w: review output schema for %q is missing", ErrValidation, node.ID)
		}
		return cloneRawMessage(snapshot.OutputSchema), nil
	}
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionRunAgent:
		var params dsl.RunAgentParams
		if err := node.Params.Decode(&params); err != nil || len(params.OutputSchema) == 0 {
			return nil, fmt.Errorf("%w: review output schema for %q is missing", ErrValidation, node.ID)
		}
		return json.Marshal(params.OutputSchema)
	case dsl.ActionGoal:
		var params dsl.GoalParams
		if err := node.Params.Decode(&params); err != nil || params.OutputSchema == nil {
			return nil, fmt.Errorf("%w: review output schema for %q is missing", ErrValidation, node.ID)
		}
		return json.Marshal(*params.OutputSchema)
	case dsl.ActionRunLoop:
		return json.RawMessage(`{"type":"object"}`), nil
	case dsl.ActionTransform:
		var params dsl.TransformParams
		if err := node.Params.Decode(&params); err != nil || len(params.Map) == 0 {
			return nil, fmt.Errorf("%w: review output schema for %q is missing", ErrValidation, node.ID)
		}
		properties := make(map[string]any, len(params.Map))
		for key := range params.Map {
			properties[key] = map[string]any{}
		}
		return json.Marshal(map[string]any{"type": "object", "properties": properties})
	default:
		return nil, fmt.Errorf("%w: review output schema for %q is missing", ErrValidation, node.ID)
	}
}

func inferredReviewSchema(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		properties := make(map[string]any, len(typed))
		required := make([]string, 0, len(typed))
		for key, child := range typed {
			properties[key] = inferredReviewSchema(child)
			required = append(required, key)
		}
		sort.Strings(required)
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	case []any:
		items := map[string]any{}
		if len(typed) > 0 {
			items = inferredReviewSchema(typed[0])
		}
		return map[string]any{"type": "array", "items": items}
	case string:
		return map[string]any{"type": "string"}
	case bool:
		return map[string]any{"type": "boolean"}
	case float64, float32:
		return map[string]any{"type": "number"}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return map[string]any{"type": "integer"}
	case nil:
		return map[string]any{"type": "null"}
	default:
		return map[string]any{}
	}
}

func reviewExpiresAt(review *dsl.ReviewSpec, fallback string, now time.Time) (*time.Time, error) {
	expiry := review.Expires
	if expiry == nil && strings.TrimSpace(fallback) != "" {
		expiry = &dsl.WaitExpiry{After: fallback}
	}
	if expiry == nil {
		return nil, nil
	}
	duration, err := time.ParseDuration(strings.TrimSpace(expiry.After))
	if err != nil || duration <= 0 {
		return nil, fmt.Errorf("%w: expiry is invalid", ErrValidation)
	}
	value := now.Add(duration)
	return &value, nil
}
