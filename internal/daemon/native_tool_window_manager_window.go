package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

func (n *daemonNativeTools) windowList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowListInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	snapshot, err := n.windowManagerSnapshot(ctx, scope, req.ToolID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	windows := windowManagerSortedWindows(
		snapshot,
		windowmanager.DesktopID(input.DesktopID),
		input.IncludeMinimized,
	)
	payload := windowManagerWindowsResult{
		WorkspaceID: snapshot.WorkspaceID,
		Revision:    snapshot.Revision,
		Windows:     windows,
	}
	payload.Deprecations = []windowmanager.Deprecation{}
	if input.IncludeMinimized {
		payload.Deprecations = append(
			payload.Deprecations,
			windowmanager.SurfaceDeprecation("compozy__window_list.include_minimized"),
		)
		windowManagerSurfaceDeprecationLogger(slog.Default(), &deprecatedWindowSurfaces)(ctx,
			windowmanager.CommandRequest{Actor: windowManagerActor(scope)}, payload.Deprecations)
	}
	return structuredResult(payload, fmt.Sprintf("listed %d windows at revision %d", len(windows), payload.Revision))
}

func (n *daemonNativeTools) windowOpen(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowOpenInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	app, _ := windowmanager.RetiredApp(strings.TrimSpace(input.App))
	if strings.TrimSpace(input.RestoreWindowID) == "" && !corecmds.RegisteredApp(app) {
		return toolspkg.ToolResult{}, windowManagerInvalidInput(
			req.ToolID,
			fmt.Errorf("unregistered window app %q: %w", app, windowmanager.ErrInvalidCommand),
		)
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowNavigate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowNavigateInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowClose(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowCloseInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowFocus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowFocusInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowMove(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowMoveInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := input.validate(); err != nil {
		return toolspkg.ToolResult{}, windowManagerInvalidInput(req.ToolID, err)
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowResize(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowResizeInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowSwap(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowSwapInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowFloat(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowFloatInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowZoom(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowZoomInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}
