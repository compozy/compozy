package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
)

func (n *daemonNativeTools) windowManagerService() windowmanager.Service {
	if n == nil || n.deps == nil {
		return nil
	}
	return n.deps.WindowManager
}

func (n *daemonNativeTools) windowManagerSnapshot(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	workspaceRef string,
) (windowmanager.Snapshot, error) {
	service := n.windowManagerService()
	if service == nil {
		return windowmanager.Snapshot{}, nativeUnavailableError(id, "window manager is unavailable")
	}
	workspaceID, err := n.windowManagerWorkspaceID(ctx, id, workspaceRef, scope)
	if err != nil {
		return windowmanager.Snapshot{}, err
	}
	snapshot, err := service.Snapshot(ctx, workspaceID)
	if err != nil {
		return windowmanager.Snapshot{}, windowManagerToolError(id, err)
	}
	return snapshot, nil
}

func (n *daemonNativeTools) executeWindowManagerCommand(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input windowManagerMutationInput,
	command windowmanager.Command,
) (toolspkg.ToolResult, error) {
	service := n.windowManagerService()
	if service == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "window manager is unavailable")
	}
	request, err := n.windowManagerCommandRequest(ctx, scope, req, input, command)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := service.Execute(ctx, request)
	if err != nil {
		return toolspkg.ToolResult{}, windowManagerToolError(req.ToolID, err)
	}
	payload := newWindowManagerCommandResult(command.CommandID(), result)
	return structuredResult(payload, fmt.Sprintf("%s at revision %d", command.CommandID(), payload.Revision))
}

func (n *daemonNativeTools) previewWindowManagerCommand(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input windowManagerMutationInput,
	command windowmanager.Command,
) (toolspkg.ToolResult, error) {
	service := n.windowManagerService()
	if service == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "window manager is unavailable")
	}
	request, err := n.windowManagerCommandRequest(ctx, scope, req, input, command)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	preview, err := service.Preview(ctx, request)
	if err != nil {
		return toolspkg.ToolResult{}, windowManagerToolError(req.ToolID, err)
	}
	payload := newWindowManagerPreviewResult(command.CommandID(), preview)
	return structuredResult(payload, fmt.Sprintf("previewed %s at revision %d", command.CommandID(), payload.Revision))
}
