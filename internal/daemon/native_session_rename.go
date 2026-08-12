package daemon

import (
	"context"
	"errors"

	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionRenameInput struct {
	Workspace string `json:"workspace,omitempty"`
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
}

func (n *daemonNativeTools) sessionRename(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionRenameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, workspaceID, input.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionRenameManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: session rename manager is required")
	}
	info, err := manager.Rename(ctx, workspaceID, input.SessionID, input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}
