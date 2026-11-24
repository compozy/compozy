package daemon

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) controllerComputerBindings() map[toolspkg.ToolID]nativeToolBinding {
	registry := n.dependencyAvailability(func() bool { return n.computersService() != nil })
	setup := n.dependencyAvailability(func() bool { return n.sshSetupService() != nil })
	account := n.dependencyAvailability(func() bool { return n.deps.Account != nil })
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDComputerList:        {call: controllerComputerCall(n.computerList), availability: registry},
		toolspkg.ToolIDComputerStatus:      {call: controllerComputerCall(n.computerStatus), availability: registry},
		toolspkg.ToolIDComputerAdd:         {call: controllerComputerCall(n.computerAdd), availability: registry},
		toolspkg.ToolIDComputerRemove:      {call: controllerComputerCall(n.computerRemove), availability: registry},
		toolspkg.ToolIDComputerSetupStatus: {call: controllerComputerCall(n.computerSetupStatus), availability: setup},
		toolspkg.ToolIDComputerSetupAnswer: {call: controllerComputerCall(n.computerSetupAnswer), availability: setup},
		toolspkg.ToolIDAccountStatus:       {call: controllerComputerCall(n.accountStatus), availability: account},
	}
}

func controllerComputerCall(call toolspkg.NativeToolFunc) toolspkg.NativeToolFunc {
	return func(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
		if _, remote := remotehost.PrincipalFromContext(ctx); remote {
			return toolspkg.ToolResult{}, nativeComputerToolError(req.ToolID, toolspkg.ErrorCodeNotForwardable,
				"This tool is available on the controller.", http.StatusBadRequest, nil)
		}
		result, err := call(ctx, scope, req)
		return result, asComputerToolError(req.ToolID, err)
	}
}

func (n *daemonNativeTools) computersService() core.ComputersService {
	if n.deps.Computers == nil {
		return nil
	}
	return n.deps.Computers()
}

func (n *daemonNativeTools) computerList(
	ctx context.Context,
	_ toolspkg.Scope,
	_ toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	result, err := core.ComputerList(ctx, n.computersService())
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(result, "Listed computers")
}

type nativeComputerRef struct {
	ID     string `json:"id"`
	Unlink bool   `json:"unlink"`
}

func (n *daemonNativeTools) computerStatus(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeComputerRef
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	view, err := computers.ResolveReference(ctx, n.computersService().List, input.ID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(core.ComputerDTO(view), "Read computer status")
}

func (n *daemonNativeTools) computerRemove(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeComputerRef
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	service := n.computersService()
	view, err := computers.ResolveReference(ctx, service.List, input.ID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if _, err := service.Remove(ctx, view.ID, computers.RemoveOptions{Unlink: input.Unlink}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(struct {
		Removed bool `json:"removed"`
	}{Removed: true}, "Removed computer")
}

func (n *daemonNativeTools) computerAdd(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		ID remoteproto.ComputerID `json:"id"`
		contract.AddComputerRequest
	}
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if (input.Kind == contract.ComputerAddKindAccount && input.ID == "") ||
		(input.Kind == contract.ComputerAddKindSSH && input.Host == "") {
		return toolspkg.ToolResult{}, nativeComputerToolError(req.ToolID, toolspkg.ErrorCodeInvalidInput,
			"Provide id for an account computer or host for SSH setup.", http.StatusBadRequest, nil)
	}
	if input.Kind == contract.ComputerAddKindAccount {
		view, err := n.computersService().AddFromAccount(ctx, input.ID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(core.ComputerDTO(view), "Added account computer")
	}
	service := n.sshSetupService()
	if service == nil {
		return toolspkg.ToolResult{}, nativeComputerToolError(req.ToolID, toolspkg.ErrorCodeUnavailable,
			"SSH setup is unavailable.", http.StatusServiceUnavailable, nil)
	}
	id, err := service.Start(ctx, sshsetup.Request{
		Host:             input.Host,
		User:             input.User,
		Port:             input.Port,
		Name:             input.Name,
		TrustFingerprint: input.TrustHostFingerprint,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	status, err := service.Status(ctx, id, 0)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(core.ComputerSetupDTO(status), "Started SSH setup")
}

type nativeComputerSetupRequest struct {
	SetupID string `json:"setup_id"`
	WaitMS  int    `json:"wait_ms"`
	Accept  bool   `json:"accept"`
}

func (n *daemonNativeTools) computerSetupStatus(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeComputerSetupRequest
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	status, err := n.sshSetupService().Status(ctx, input.SetupID, time.Duration(input.WaitMS)*time.Millisecond)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(core.ComputerSetupDTO(status), "Read SSH setup status")
}

func (n *daemonNativeTools) computerSetupAnswer(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeComputerSetupRequest
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	answer := sshsetup.Answer{Kind: sshsetup.TrustHost, Accept: input.Accept}
	if err := n.sshSetupService().Answer(ctx, input.SetupID, answer, false); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(struct{}{}, "Answered SSH host trust prompt")
}

func (n *daemonNativeTools) accountStatus(
	_ context.Context,
	_ toolspkg.Scope,
	_ toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return structuredResult(core.AccountStatus(n.deps.Account.Status()).CLIStatus(), "Read account status")
}
