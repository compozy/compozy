package daemon

import (
	"context"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) windowGroup(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowGroupInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowReorder(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowReorderInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowActivate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowActivateInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowPin(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowPinInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) windowReopen(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWindowReopenInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}
