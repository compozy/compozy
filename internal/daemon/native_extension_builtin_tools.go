package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// extensionRestoreToolInput is the restore tool's argument. Built-ins have no owner selector.
type extensionRestoreToolInput struct {
	Name string `json:"name"`
}

// extensionBuiltinToolBindings binds the restore and removed-list tools. Permanent removal is intentionally
// not bound: it has no tool ID, so an agent cannot delete a built-in permanently (Q5 a).
func (n *daemonNativeTools) extensionBuiltinToolBindings(
	availability *toolspkg.NativeAvailabilityGroup,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDExtensionsRestore: {
			call:         n.extensionRestore,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsRemovedList: {
			call:         n.extensionRemovedList,
			availability: availability,
		},
	}
}

// extensionRestore restores one built-in with the same write authority as other extension writes. Its
// structured result is the payload the CLI prints for `extension restore -o json`.
func (n *daemonNativeTools) extensionRestore(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionRestoreToolInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return toolspkg.ToolResult{}, nativeExtensionValidationError(req.ToolID, errors.New("name is required"))
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	item, err := n.extensionService().RestoreBuiltin(ctx, name, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(item, item.Name)
}

// extensionRemovedList returns the removed-built-in listing. An empty listing is `{"removed":[]}`, the
// same bytes the CLI and HTTP surfaces produce.
func (n *daemonNativeTools) extensionRemovedList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	items, err := n.extensionService().ListRemovedBuiltins(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	if items == nil {
		items = []contract.ExtensionRemovedBuiltinPayload{}
	}
	return structuredResult(
		contract.ExtensionRemovedBuiltinsResponse{Removed: items},
		fmt.Sprintf("%d removed built-ins", len(items)),
	)
}
