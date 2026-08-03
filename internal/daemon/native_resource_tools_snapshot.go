package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) resourcesSnapshot(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	records, err := n.readResourcePayloads(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := map[string]any{
		"count":   len(records),
		"records": records,
	}
	return structuredResult(payload, fmt.Sprintf("%d resources", len(records)))
}
