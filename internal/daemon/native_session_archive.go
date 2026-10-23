package daemon

import (
	"context"
	"errors"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionArchiveInput struct {
	Workspace string `json:"workspace,omitempty"`
	SessionID string `json:"session_id"`
	Stop      bool   `json:"stop,omitempty"`
}

func (n *daemonNativeTools) sessionArchive(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.setSessionArchived(ctx, scope, req, true)
}

func (n *daemonNativeTools) sessionUnarchive(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.setSessionArchived(ctx, scope, req, false)
}

func (n *daemonNativeTools) setSessionArchived(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	archived bool,
) (toolspkg.ToolResult, error) {
	var input sessionArchiveInput
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
	// Stopping the caller would cancel the turn executing this call, as session_stop refuses.
	if archived && input.Stop && strings.TrimSpace(input.SessionID) == strings.TrimSpace(scope.SessionID) {
		return toolspkg.ToolResult{}, nativeSelfTargetError(req.ToolID)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, workspaceID, input.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionArchiveManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: session archive manager is required")
	}
	var info *session.Info
	switch {
	case archived && input.Stop:
		info, err = manager.ArchiveWithStop(ctx, workspaceID, input.SessionID)
	case archived:
		info, err = manager.Archive(ctx, workspaceID, input.SessionID)
	default:
		info, err = manager.Unarchive(ctx, workspaceID, input.SessionID)
	}
	if errors.Is(err, session.ErrSessionArchiveStopFailed) {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeSessionArchiveStopFailed,
			req.ToolID,
			err.Error(),
			err,
		)
	}
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}
