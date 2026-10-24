package daemon

import (
	"context"
	"errors"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionReparentInput struct {
	Workspace       string `json:"workspace,omitempty"`
	SessionID       string `json:"session_id"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
	TopLevel        bool   `json:"top_level,omitempty"`
}

func (n *daemonNativeTools) sessionReparent(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionReparentInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	parentID := strings.TrimSpace(input.ParentSessionID)
	if (parentID == "") == !input.TopLevel {
		return toolspkg.ToolResult{}, nativeInputError(
			req.ToolID, errors.New("set exactly one of parent_session_id or top_level"),
		)
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	if err := n.nativeReparentTarget(ctx, scope, req.ToolID, workspaceID, input.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionReparentManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: session reparent manager is required")
	}
	if input.TopLevel {
		info, err := manager.DetachSession(ctx, workspaceID, input.SessionID)
		if err != nil {
			return toolspkg.ToolResult{}, nativeReparentError(req.ToolID, err)
		}
		payload := core.SessionPayloadFromInfo(info)
		return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
	}
	if err := n.nativeReparentTarget(ctx, scope, req.ToolID, workspaceID, parentID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	info, err := manager.AdoptSession(ctx, workspaceID, input.SessionID, parentID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeReparentError(req.ToolID, err)
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

func (n *daemonNativeTools) nativeReparentTarget(
	ctx context.Context, scope toolspkg.Scope, id toolspkg.ToolID, workspaceID, sessionID string,
) error {
	info, err := n.nativeSessionInWorkspace(ctx, scope, id, workspaceID, sessionID)
	if err != nil {
		return err
	}
	profileID, _, _, err := n.nativeCurrentProfileIdentity(ctx, scope)
	if err != nil {
		return err
	}
	targetProfile := strings.TrimSpace(info.ProfileID)
	if targetProfile == "" {
		targetProfile = store.DefaultProfileID
	}
	if targetProfile != profileID {
		return nativeWorkspaceAccessDeniedError(id)
	}
	return nil
}

func nativeReparentError(id toolspkg.ToolID, err error) error {
	if code := core.SessionReparentErrorCode(err); code != "" {
		return toolspkg.NewToolError(toolspkg.ErrorCode(code), id, err.Error(), err)
	}
	return nativeHTTPStatusToolError(id, err, core.StatusForSessionError(err))
}
