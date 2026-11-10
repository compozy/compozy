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
	return (resultViews[resourceListResult]{
		Compact: compactResources,
		Preview: func(resourceListResult) string { return fmt.Sprintf("%d resources", len(records)) },
	}).result(req.View, resourceListResult{Count: new(len(records)), Records: records})
}
