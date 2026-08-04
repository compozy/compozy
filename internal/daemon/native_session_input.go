package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionInputListInput struct {
	Workspace string `json:"workspace,omitempty"`
	SessionID string `json:"session_id"`
}

type sessionInputReplaceInput struct {
	Workspace      string `json:"workspace,omitempty"`
	SessionID      string `json:"session_id"`
	QueueEntryID   string `json:"queue_entry_id"`
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type sessionInputCancelInput struct {
	Workspace    string `json:"workspace,omitempty"`
	SessionID    string `json:"session_id"`
	QueueEntryID string `json:"queue_entry_id"`
}

type sessionInputPromoteInput struct {
	Workspace      string `json:"workspace,omitempty"`
	SessionID      string `json:"session_id"`
	QueueEntryID   string `json:"queue_entry_id"`
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
	ExpectedTurnID string `json:"expected_turn_id"`
}

func (n *daemonNativeTools) sessionInputsList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionInputListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := n.nativeSessionInputScope(ctx, scope, req.ToolID, input.Workspace, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	inputs, err := n.deps.Sessions.ListPendingInputs(ctx, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payloads := make([]contract.SessionInputPayload, 0, len(inputs))
	for _, pendingInput := range inputs {
		payloads = append(payloads, core.SessionInputPayloadFromSession(pendingInput))
	}
	return structuredResult(
		contract.SessionInputListResponse{Inputs: payloads},
		fmt.Sprintf("%d pending session inputs", len(payloads)),
	)
}

func (n *daemonNativeTools) sessionInputReplace(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionInputReplaceInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	text, err := requiredNativeString(req.ToolID, nativeToolsTextKey, input.Text)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	messageID, err := requiredNativeString(req.ToolID, "message_id", input.MessageID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	idempotencyKey, err := requiredNativeString(req.ToolID, "idempotency_key", input.IdempotencyKey)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, entryID, err := n.nativeSessionInputMutationScope(
		ctx, scope, req.ToolID, input.Workspace, input.SessionID, input.QueueEntryID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	updated, err := n.deps.Sessions.ReplacePendingInput(ctx, sessionID, entryID, session.ReplacePendingInputOpts{
		Text: text, MessageID: messageID, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionInputPayloadFromSession(updated)
	return structuredResult(contract.SessionInputResponse{Input: payload}, payload.ID)
}

func (n *daemonNativeTools) sessionInputCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionInputCancelInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, entryID, err := n.nativeSessionInputMutationScope(
		ctx, scope, req.ToolID, input.Workspace, input.SessionID, input.QueueEntryID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := n.deps.Sessions.CancelQueuedPrompt(ctx, sessionID, entryID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.nativeSessionPromptResult(result)
}

func (n *daemonNativeTools) sessionInputPromote(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionInputPromoteInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	text, err := requiredNativeString(req.ToolID, nativeToolsTextKey, input.Text)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	messageID, err := requiredNativeString(req.ToolID, "message_id", input.MessageID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	idempotencyKey, err := requiredNativeString(req.ToolID, "idempotency_key", input.IdempotencyKey)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	expectedTurnID, err := requiredNativeString(req.ToolID, "expected_turn_id", input.ExpectedTurnID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, entryID, err := n.nativeSessionInputMutationScope(
		ctx, scope, req.ToolID, input.Workspace, input.SessionID, input.QueueEntryID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := n.deps.Sessions.PromotePendingInputToSteer(
		ctx,
		sessionID,
		entryID,
		session.PromotePendingInputOpts{
			Text: text, MessageID: messageID, IdempotencyKey: idempotencyKey,
			ExpectedTurnID: expectedTurnID,
		},
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.nativeSessionPromptResult(result)
}

func (n *daemonNativeTools) nativeSessionInputScope(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	workspaceRef string,
	sessionRef string,
) (string, error) {
	sessionID, err := requiredNativeString(toolID, "session_id", sessionRef)
	if err != nil {
		return "", err
	}
	return n.nativeSessionInputWorkspaceScope(ctx, scope, toolID, workspaceRef, sessionID)
}

func (n *daemonNativeTools) nativeSessionInputWorkspaceScope(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	workspaceRef string,
	sessionID string,
) (string, error) {
	resolved, err := n.nativeResolvedWorkspace(ctx, toolID, workspaceRef, scope)
	if err != nil {
		return "", err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return "", nativeNetworkInputError(toolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, toolID, workspaceID, sessionID); err != nil {
		return "", err
	}
	return sessionID, nil
}

func (n *daemonNativeTools) nativeSessionInputMutationScope(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	workspaceRef string,
	sessionRef string,
	entryRef string,
) (string, string, error) {
	sessionID, err := requiredNativeString(toolID, "session_id", sessionRef)
	if err != nil {
		return "", "", err
	}
	entryID, err := requiredNativeString(toolID, "queue_entry_id", entryRef)
	if err != nil {
		return "", "", err
	}
	sessionID, err = n.nativeSessionInputWorkspaceScope(ctx, scope, toolID, workspaceRef, sessionID)
	if err != nil {
		return "", "", err
	}
	return sessionID, entryID, nil
}

func (n *daemonNativeTools) nativeSessionPromptResult(
	result session.SendPromptResult,
) (toolspkg.ToolResult, error) {
	payload, err := core.PromptResultPayloadFromSession(result)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"prompt": payload}, payload.Status)
}
