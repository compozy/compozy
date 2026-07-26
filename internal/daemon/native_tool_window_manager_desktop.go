package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) desktopList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWorkspaceInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	snapshot, err := n.windowManagerSnapshot(ctx, scope, req.ToolID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := windowManagerDesktopsResult{
		WorkspaceID: snapshot.WorkspaceID,
		Revision:    snapshot.Revision,
		Desktops:    snapshot.Desktops,
	}
	return structuredResult(
		payload,
		fmt.Sprintf("listed %d desktops at revision %d", len(payload.Desktops), payload.Revision),
	)
}

func (n *daemonNativeTools) desktopCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerDesktopCreateInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) desktopUpdate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerDesktopUpdateInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) desktopReorder(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerDesktopReorderInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) desktopSwitch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerDesktopSwitchInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) desktopDelete(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerDesktopDeleteInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) desktopClients(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWorkspaceInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	snapshot, err := n.windowManagerSnapshot(ctx, scope, req.ToolID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	clients, err := n.windowManagerService().Clients(ctx, snapshot.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, windowManagerToolError(req.ToolID, err)
	}
	payload := windowManagerClientsResult{
		WorkspaceID: snapshot.WorkspaceID,
		Revision:    snapshot.Revision,
		Clients:     clients,
	}
	return structuredResult(payload, fmt.Sprintf("listed %d desktop clients", len(payload.Clients)))
}
