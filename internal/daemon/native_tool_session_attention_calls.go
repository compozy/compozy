package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type operatorNotificationPublisher interface {
	PublishOperatorNotification(context.Context, session.NotifyRequest) (session.NotifyResult, error)
}

type nativeNotifyInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (n *daemonNativeTools) sessionAttentionToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDNotify: {call: n.notifyOperator, availability: availability},
	}
}

func (n *daemonNativeTools) notifyOperator(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeNotifyInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", scope.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, "", scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, req.ToolID, workspaceID, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	publisher, ok := n.deps.Sessions.(operatorNotificationPublisher)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: operator notification service is required")
	}
	result, err := publisher.PublishOperatorNotification(ctx, session.NotifyRequest{
		SessionID: info.ID, WorkspaceID: info.WorkspaceID,
		Title: input.Title, Body: input.Body,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{
		nativePayloadOutcomeKey: result.Outcome,
		"retry_after_ms":        result.RetryAfterMS,
	}, string(result.Outcome))
}
