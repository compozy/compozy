package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) terminalClose(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	exit, err := manager.Close(ctx, workspaceID, terminalpkg.ID(input.TerminalID), actor, terminalpkg.SignalHUP)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"exit": exit}, "terminal closed")
}

func (n *daemonNativeTools) terminalList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	items, err := manager.List(ctx, workspaceID, store.ReadScope{ProfileID: actor.ProfileID})
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	projected, err := n.terminalToolInfos(ctx, items)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"terminals": projected}, fmt.Sprintf("%d terminals", len(projected)))
}

func (n *daemonNativeTools) terminalRequestInput(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalInputRequestInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	outcome, err := handle.RequestInput(ctx, terminalpkg.InputRequest{
		Reason: input.Reason, PromptExcerpt: input.PromptExcerpt, Redact: input.Redact,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return structuredResult(outcome, "terminal input request resolved")
}

func (n *daemonNativeTools) terminalYield(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalYieldInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err == nil {
		err = handle.Yield(ctx, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeToolsLeaseStateKey: handle.Info().Lease}, "terminal control yielded")
}

func (n *daemonNativeTools) terminalClaim(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	claimer, ok := manager.(interface {
		Claim(context.Context, string, terminalpkg.ID, terminalpkg.Actor) error
	})
	if !ok {
		return toolspkg.ToolResult{}, nativeCommandDependencyError(req.ToolID, "terminal claim is unavailable")
	}
	if err := claimer.Claim(ctx, workspaceID, terminalpkg.ID(input.TerminalID), actor); err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	info, err := manager.Get(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{"granted": true, nativeToolsLeaseStateKey: info.Lease},
		"terminal control claimed",
	)
}
