package loop

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const requestPreviewLimitBytes = 16 * 1024

func evaluateAskNode(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	outputs []GenerationOutput,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	var params dsl.AskParams
	if err := node.Params.Decode(&params); err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: decode ask node %q: %w", node.ID, err)
	}
	namespace, err := runtimeNamespaceWithHistory(
		eval.run, eval.generation, eval.resolved.Definition.Graph, eval.topology,
		outputs, eval.history, node.ID, output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	renderedPrompt, err := renderActionParam("nodes."+string(node.ID)+".params.prompt", params.Prompt, namespace)
	if err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: render ask prompt %q: %w", node.ID, err)
	}
	prompt, ok := renderedPrompt.(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return GenerationOutput{}, nil, fmt.Errorf("%w: ask node %q prompt must resolve to a string", ErrValidation, node.ID)
	}
	renderedContext, err := renderActionParam(
		"nodes."+string(node.ID)+".params.context", params.Context, namespace,
	)
	if err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: render ask context %q: %w", node.ID, err)
	}
	contextMap, ok := renderedContext.(map[string]any)
	if !ok {
		return GenerationOutput{}, nil, fmt.Errorf("%w: ask node %q context must resolve to an object", ErrValidation, node.ID)
	}
	contextRaw, previewRaw, err := requestContextPayloads(contextMap)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	expectRaw, err := json.Marshal(params.Expect)
	if err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: marshal ask expect %q: %w", node.ID, err)
	}
	decisionsRaw := json.RawMessage(`["respond"]`)
	now := eval.now.UTC()
	var expiry = params.Expires
	if expiry == nil && strings.TrimSpace(eval.effective.RequestExpireAfter) != "" {
		expiry = &dsl.WaitExpiry{After: eval.effective.RequestExpireAfter}
	}
	var expiresAt *time.Time
	if expiry != nil {
		duration, parseErr := time.ParseDuration(strings.TrimSpace(expiry.After))
		if parseErr != nil || duration <= 0 {
			return GenerationOutput{}, nil, fmt.Errorf("%w: ask node %q expiry is invalid", ErrValidation, node.ID)
		}
		value := now.Add(duration)
		expiresAt = &value
	}
	output.Status = generationOutputWaiting
	output.TaskRunID = ""
	output.NextAttemptAt = nil
	output.Epoch++
	wait := NodeWaitIntent{
		NodeID: NodeID(output.NodeID), ItemIndex: output.ItemIndex, Kind: NodeWaitKindRequest,
		NextEscalationAt: expiresAt, Expect: expectRaw, IssuedEpoch: output.Epoch, CreatedAt: now,
	}
	agents := dsl.ResponderAgentsDeny
	if params.Responders != nil && params.Responders.Agents != "" {
		agents = params.Responders.Agents
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	payload.Waits = append(payload.Waits, wait)
	payload.Requests = append(payload.Requests, RequestIntent{
		WorkspaceID: eval.run.WorkspaceID, NodeID: node.ID, ItemIndex: output.ItemIndex,
		Kind: RequestKindAsk, Prompt: diagnostics.RedactAndBound(prompt, requestPreviewLimitBytes),
		Context: contextRaw, ContextPreview: previewRaw, AnswerSchema: expectRaw,
		Decisions: decisionsRaw, Agents: agents, ExpiresAt: expiresAt,
		IssuedEpoch: output.Epoch, OpenedAt: now,
	})
	payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
		Kind: GenerationLifecycleEventNodeWaitStarted, NodeID: output.NodeID,
		ItemIndex: output.ItemIndex, Attempt: output.Attempt, IssuedEpoch: output.Epoch,
		WaitKind: NodeWaitKindRequest, NextAttemptAt: expiresAt,
	})
	plan.Snapshot.Payload = payload
	return output, nil, nil
}

func requestContextPayloads(contextMap map[string]any) (json.RawMessage, json.RawMessage, error) {
	redacted := diagnostics.RedactValue(contextMap)
	full, err := json.Marshal(redacted)
	if err != nil {
		return nil, nil, fmt.Errorf("loop: marshal redacted request context: %w", err)
	}
	if len(full) <= requestPreviewLimitBytes {
		return full, cloneRawMessage(full), nil
	}
	preview, err := json.Marshal(map[string]any{
		"truncated": true,
		"byte_size": len(full),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("loop: marshal request context preview: %w", err)
	}
	return full, preview, nil
}
