package daemon

import (
	"context"
	"errors"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionMarkSeenInput struct {
	Workspace string `json:"workspace,omitempty"`
	SessionID string `json:"session_id"`
}

func (n *daemonNativeTools) sessionMarkSeen(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionMarkSeenInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, workspaceID, input.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionAttentionSeenManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: session attention manager is required")
	}
	info, err := manager.MarkAttentionSeen(context.WithoutCancel(ctx), input.SessionID)
	if errors.Is(err, session.ErrAttentionRequiresAction) {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeAttentionRequiresAction, req.ToolID, err.Error(), err,
		)
	}
	if errors.Is(err, session.ErrAttentionChanged) {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(toolspkg.ErrorCodeConflict, req.ToolID, err.Error(), err)
	}
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(info)
	return sessionRecordViews.result(req.View, payload)
}
