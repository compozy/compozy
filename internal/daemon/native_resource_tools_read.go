package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) readResourcePayloads(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) ([]contract.ResourceRecordPayload, error) {
	filter, err := decodeResourceFilterInput(req, scope)
	if err != nil {
		return nil, err
	}
	records, err := n.deps.Resources.List(ctx, filter)
	if err != nil {
		return nil, nativeResourceToolError(req.ToolID, err)
	}
	return core.ResourceRecordPayloadsFromRaw(records), nil
}
