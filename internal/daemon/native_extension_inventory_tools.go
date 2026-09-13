package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
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
	name, err := requiredNativeExtensionName(req.ToolID, input.Name, input.Owner)
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

func (n *daemonNativeTools) extensionList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		Owner string `json:"owner"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	owner := strings.TrimSpace(input.Owner)
	if owner != "" {
		if _, err := requiredNativeExtensionName(
			req.ToolID,
			strings.TrimPrefix(owner, "extension:"),
			owner,
		); err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	service := n.extensionService()
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	var items []contract.ExtensionPayload
	if nativeExtensionDefaultRead(actor) {
		items, err = service.List(ctx)
	} else {
		items, err = service.ListScoped(ctx, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	if owner != "" {
		selected := make([]contract.ExtensionPayload, 0, 1)
		for _, item := range items {
			if "extension:"+item.Name == owner {
				selected = append(selected, item)
			}
		}
		items = selected
	}
	return structuredResult(
		map[string]any{nativeExtensionToolsExtensionsKey: items},
		fmt.Sprintf("%d installed extensions", len(items)),
	)
}

func (n *daemonNativeTools) extensionInfo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeExtensionName(req.ToolID, input.Name, input.Owner)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	var item contract.ExtensionPayload
	if nativeExtensionDefaultRead(actor) {
		item, err = n.extensionService().Status(ctx, name)
	} else {
		item, err = n.extensionService().StatusScoped(ctx, name, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeExtensionToolsExtensionKey: item}, item.Name)
}

func nativeExtensionDefaultRead(actor taskpkg.ActorContext) bool {
	profileID := strings.TrimSpace(actor.ReadScope.ProfileID)
	return strings.TrimSpace(actor.Scope.WorkspaceID) == "" && (profileID == "" || profileID == store.DefaultProfileID)
}
