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
	namespace, err := runtimeNamespaceWithHistory(
		eval.run, eval.generation, eval.resolved.Definition.Graph, eval.topology,
		outputs, eval.history, node.ID, output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	request, err := materializeAskRequest(node, namespace, eval.effective.RequestExpireAfter, eval.now.UTC())
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	output.Status = generationOutputWaiting
	output.TaskRunID = ""
	output.NextAttemptAt = nil
	output.Epoch++
	wait := NodeWaitIntent{
		NodeID: NodeID(output.NodeID), ItemIndex: output.ItemIndex, Kind: NodeWaitKindRequest,
		NextEscalationAt: request.expiresAt, Expect: request.expect,
		IssuedEpoch: output.Epoch, CreatedAt: request.openedAt,
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	payload.Waits = append(payload.Waits, wait)
	payload.Requests = append(payload.Requests, RequestIntent{
		WorkspaceID: eval.run.WorkspaceID, NodeID: node.ID, ItemIndex: output.ItemIndex,
		Kind: RequestKindAsk, Prompt: diagnostics.RedactAndBound(request.prompt, requestPreviewLimitBytes),
		Context: request.context, ContextPreview: request.preview, AnswerSchema: request.expect,
		Decisions: json.RawMessage(`["respond"]`), Agents: request.agents, ExpiresAt: request.expiresAt,
		IssuedEpoch: output.Epoch, OpenedAt: request.openedAt,
	})
	payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
		Kind: GenerationLifecycleEventNodeWaitStarted, NodeID: output.NodeID,
		ItemIndex: output.ItemIndex, Attempt: output.Attempt, IssuedEpoch: output.Epoch,
		WaitKind: NodeWaitKindRequest, NextAttemptAt: request.expiresAt,
	})
	plan.Snapshot.Payload = payload
	return output, nil, nil
}

type askRequest struct {
	prompt    string
	context   json.RawMessage
	preview   json.RawMessage
	expect    json.RawMessage
	agents    dsl.ResponderAgentPolicy
	expiresAt *time.Time
	openedAt  time.Time
}

func materializeAskRequest(
	node dsl.Node,
	namespace map[string]any,
	defaultExpiry string,
	now time.Time,
) (askRequest, error) {
	var params dsl.AskParams
	if err := node.Params.Decode(&params); err != nil {
		return askRequest{}, fmt.Errorf("loop: decode ask node %q: %w", node.ID, err)
	}
	renderedPrompt, err := renderActionParam("nodes."+string(node.ID)+".params.prompt", params.Prompt, namespace)
	if err != nil {
		return askRequest{}, fmt.Errorf("loop: render ask prompt %q: %w", node.ID, err)
	}
	prompt, ok := renderedPrompt.(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return askRequest{}, fmt.Errorf("%w: ask node %q prompt must resolve to a string", ErrValidation, node.ID)
	}
	renderedContext, err := renderActionParam(
		"nodes."+string(node.ID)+".params.context", params.Context, namespace,
	)
	if err != nil {
		return askRequest{}, fmt.Errorf("loop: render ask context %q: %w", node.ID, err)
	}
	if renderedContext == nil {
		renderedContext = map[string]any{}
	}
	contextMap, ok := renderedContext.(map[string]any)
	if !ok {
		return askRequest{}, fmt.Errorf("%w: ask node %q context must resolve to an object", ErrValidation, node.ID)
	}
	contextRaw, previewRaw, err := requestContextPayloads(contextMap)
	if err != nil {
		return askRequest{}, err
	}
	expectRaw, err := json.Marshal(params.Expect)
	if err != nil {
		return askRequest{}, fmt.Errorf("loop: marshal ask expect %q: %w", node.ID, err)
	}
	expiresAt, err := askRequestExpiry(params.Expires, defaultExpiry, node.ID, now)
	if err != nil {
		return askRequest{}, err
	}
	agents := dsl.ResponderAgentsDeny
	if params.Responders != nil && params.Responders.Agents != "" {
		agents = params.Responders.Agents
	}
	return askRequest{
		prompt: prompt, context: contextRaw, preview: previewRaw, expect: expectRaw,
		agents: agents, expiresAt: expiresAt, openedAt: now,
	}, nil
}

func askRequestExpiry(
	expiry *dsl.WaitExpiry,
	defaultExpiry string,
	nodeID dsl.NodeID,
	now time.Time,
) (*time.Time, error) {
	if expiry == nil && strings.TrimSpace(defaultExpiry) != "" {
		expiry = &dsl.WaitExpiry{After: defaultExpiry}
	}
	if expiry == nil {
		return nil, nil
	}
	duration, err := time.ParseDuration(strings.TrimSpace(expiry.After))
	if err != nil || duration <= 0 {
		return nil, fmt.Errorf("%w: ask node %q expiry is invalid", ErrValidation, nodeID)
	}
	value := now.Add(duration)
	return &value, nil
}

func requestContextPayloads(contextMap map[string]any) (json.RawMessage, json.RawMessage, error) {
	if len(contextMap) == 0 {
		empty := json.RawMessage(`{}`)
		return empty, cloneRawMessage(empty), nil
	}
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
