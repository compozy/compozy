package daemon

import (
	"context"
	"errors"
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

// windowManagerProvider resolves one profile's window manager for agent-facing tools.
type windowManagerProvider interface {
	For(profileID string) (*windowmanager.Manager, error)
}

// windowManagerService resolves the desks of the calling session's own profile.
// An agent arranges the windows of the profile it runs under and no other (D9).
func (n *daemonNativeTools) windowManagerService(
	id toolspkg.ToolID,
	scope toolspkg.Scope,
) (windowmanager.Service, error) {
	if n == nil || n.deps == nil || n.deps.WindowManagers == nil {
		return nil, nativeUnavailableError(id, "window manager is unavailable")
	}
	manager, err := n.deps.WindowManagers.For(scope.ProfileID)
	if err != nil {
		return nil, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			"window manager profile is unresolved",
			errors.Join(toolspkg.ErrToolUnavailable, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	}
	return manager, nil
}

func (n *daemonNativeTools) windowManagerSnapshot(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	workspaceRef string,
) (windowmanager.Snapshot, error) {
	service, err := n.windowManagerService(id, scope)
	if err != nil {
		return windowmanager.Snapshot{}, err
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
	service, err := n.windowManagerService(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	request, err := n.windowManagerCommandRequest(ctx, scope, req, input, command)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := service.Execute(ctx, request)
	if err != nil {
		return toolspkg.ToolResult{}, windowManagerToolError(req.ToolID, err)
	}
	payload := newWindowManagerCommandResult(command.CommandID(), &result)
	return structuredResult(payload, fmt.Sprintf("%s at revision %d", command.CommandID(), payload.Revision))
}

func (n *daemonNativeTools) previewWindowManagerCommand(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input windowManagerMutationInput,
	command windowmanager.Command,
) (toolspkg.ToolResult, error) {
	service, err := n.windowManagerService(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
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
