package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remotehost"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) remoteHostBindings() map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDRemoteStatus: {
			call:         n.remoteHostStatus,
			availability: n.dependencyAvailability(func() bool { return n.deps.Remote != nil }),
		},
	}
}

func (n *daemonNativeTools) remoteHostStatus(
	ctx context.Context, _ toolspkg.Scope, req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if _, remote := remotehost.PrincipalFromContext(ctx); remote {
		return toolspkg.ToolResult{}, toolspkg.NewToolError("tool_not_forwardable", req.ToolID,
			"Remote status is local to this computer.", nil)
	}
	status := contract.RemoteStatusResponse(n.deps.Remote.Status())
	return structuredResult(status, "Read remote access status")
}
