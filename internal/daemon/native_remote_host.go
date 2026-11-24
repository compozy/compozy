package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/remotehost"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) remoteHostBindings() map[toolspkg.ToolID]nativeToolBinding {
	bindings := map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDRemoteStatus: {
			call:         n.remoteHostStatus,
			availability: n.dependencyAvailability(func() bool { return n.deps.Remote != nil }),
		},
		toolspkg.ToolIDComputerSetupCancel: {
			call:         n.computerSetupCancel,
			availability: n.dependencyAvailability(func() bool { return n.sshSetupService() != nil }),
		},
	}
	addNativeToolBindings(bindings, n.controllerComputerBindings())
	return bindings
}

func (n *daemonNativeTools) sshSetupService() *sshsetup.Manager {
	if n.deps.Computers == nil {
		return nil
	}
	if service, ok := n.deps.Computers().(interface{ SSHSetups() *sshsetup.Manager }); ok {
		return service.SSHSetups()
	}
	return nil
}

func (n *daemonNativeTools) computerSetupCancel(
	ctx context.Context, _ toolspkg.Scope, req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if _, remote := remotehost.PrincipalFromContext(ctx); remote {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(toolspkg.ErrorCodeNotForwardable, req.ToolID,
			"SSH setup is local to this controller.", nil)
	}
	var input struct {
		ID string `json:"setup_id"`
	}
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	service := n.sshSetupService()
	if service == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			"tool_unavailable",
			req.ToolID,
			"SSH setup is unavailable.",
			nil,
		)
	}
	if err := service.Cancel(ctx, input.ID); err != nil {
		if fault, ok := errors.AsType[*sshsetup.Failure](err); ok {
			return toolspkg.ToolResult{}, toolspkg.NewToolError(
				toolspkg.ErrorCode(fault.Code),
				req.ToolID,
				fault.Message,
				nil,
			)
		}
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.ComputerSetupCanceled{SetupID: input.ID, State: sshsetup.Canceled},
		"Stopped SSH setup and released its slot")
}

func (n *daemonNativeTools) remoteHostStatus(
	ctx context.Context, _ toolspkg.Scope, req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if _, remote := remotehost.PrincipalFromContext(ctx); remote {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(toolspkg.ErrorCodeNotForwardable, req.ToolID,
			"Remote status is local to this computer.", nil)
	}
	status := contract.RemoteStatusResponse(n.deps.Remote.Status())
	return structuredResult(status, "Read remote access status")
}
