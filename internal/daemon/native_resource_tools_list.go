package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) resourcesList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	records, err := n.readResourcePayloads(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		map[string]any{"records": records},
		fmt.Sprintf("%d resources", len(records)),
	)
}
