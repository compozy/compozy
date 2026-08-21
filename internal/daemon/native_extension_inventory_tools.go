package daemon

import (
	"context"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) extensionInventory(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	payload, err := n.extensionCoreService().InventoryScoped(ctx, name, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(payload, payload.Extension)
}
