package daemon

import (
	"context"

	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) resourcesInfo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	kind, id, err := decodeResourceInfoInput(req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := n.deps.Resources.Get(ctx, kind, id)
	if err != nil {
		return toolspkg.ToolResult{}, nativeResourceToolError(req.ToolID, err)
	}
	if !nativeResourceRecordAllowed(scope, record) {
		return toolspkg.ToolResult{}, nativeScopeMismatchError(req.ToolID, "id")
	}
	return structuredResult(map[string]any{"record": core.ResourceRecordPayloadFromRaw(record)}, id)
}
