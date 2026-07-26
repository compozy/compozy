package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
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
