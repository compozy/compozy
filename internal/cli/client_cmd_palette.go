package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type CmdPaletteClient interface {
	workspaceLookupClient
	ListCmdPaletteCommands(context.Context, string, string) (contract.CmdPaletteCommandsResponse, error)
	ListCmdPaletteClients(context.Context, string) ([]contract.CmdPaletteClient, error)
	InvokeCmdPaletteCommand(
		context.Context,
		string,
		contract.CmdPaletteInvokeRequest,
	) (contract.CmdPaletteInvokeResult, error)
	GetCmdPalettePersonalization(context.Context, string) (contract.CmdPalettePersonalizationResponse, error)
	ResetCmdPalettePersonalization(context.Context, string) (contract.CmdPalettePersonalizationResetResponse, error)
	GetPendingToolApproval(context.Context, string) (contract.ToolApprovalStatusResponse, error)
	CancelPendingToolApproval(context.Context, string) (contract.ToolApprovalStatusResponse, error)
}

type CmdPaletteMutationClient interface {
	workspaceLookupClient
	GetCmdPaletteBindings(context.Context, string) (contract.SettingsWindowManagerResponse, error)
	UpdateCmdPaletteBindings(
		context.Context,
		string,
		contract.UpdateSettingsWindowManagerRequest,
	) (contract.SettingsWindowManagerResponse, error)
	SetCmdPalettePin(context.Context, string, string, bool) (contract.CmdPalettePinResponse, error)
}

var _ CmdPaletteClient = (*daemonClient)(nil)
var _ CmdPaletteMutationClient = (*daemonClient)(nil)

func (c *daemonClient) GetCmdPaletteBindings(
	ctx context.Context,
	workspace string,
) (contract.SettingsWindowManagerResponse, error) {
	query := url.Values{
		automationScopeKey:       {string(contract.SettingsLayeredScopeWorkspace)},
		automationWorkspaceIDKey: {workspace},
	}
	var response contract.SettingsWindowManagerResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/settings/window-manager", query, nil, &response); err != nil {
		return contract.SettingsWindowManagerResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) UpdateCmdPaletteBindings(
	ctx context.Context,
	workspace string,
	request contract.UpdateSettingsWindowManagerRequest,
) (contract.SettingsWindowManagerResponse, error) {
	query := url.Values{
		automationScopeKey:       {string(contract.SettingsLayeredScopeWorkspace)},
		automationWorkspaceIDKey: {workspace},
	}
	var response contract.SettingsWindowManagerResponse
	if err := c.doJSON(
		ctx, http.MethodPatch, "/api/settings/window-manager", query, request, &response,
	); err != nil {
		return contract.SettingsWindowManagerResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) SetCmdPalettePin(
	ctx context.Context,
	workspace string,
	commandID string,
	pinned bool,
) (contract.CmdPalettePinResponse, error) {
	method := http.MethodPut
	if !pinned {
		method = http.MethodDelete
	}
	query := url.Values{cmdPaletteWorkspaceFlag: {workspace}}
	path := "/api/cmd-palette/pins/" + url.PathEscape(strings.TrimSpace(commandID))
	var response contract.CmdPalettePinResponse
	if err := c.doJSON(ctx, method, path, query, nil, &response); err != nil {
		return contract.CmdPalettePinResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) ListCmdPaletteCommands(
	ctx context.Context,
	workspace string,
	clientID string,
) (contract.CmdPaletteCommandsResponse, error) {
	query := url.Values{cmdPaletteWorkspaceFlag: {workspace}}
	if client := strings.TrimSpace(clientID); client != "" {
		query.Set("client", client)
	}
	var response contract.CmdPaletteCommandsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/cmd-palette/commands", query, nil, &response); err != nil {
		return contract.CmdPaletteCommandsResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) ListCmdPaletteClients(
	ctx context.Context,
	workspace string,
) ([]contract.CmdPaletteClient, error) {
	query := url.Values{cmdPaletteWorkspaceFlag: {workspace}}
	var response []contract.CmdPaletteClient
	if err := c.doJSON(ctx, http.MethodGet, "/api/cmd-palette/clients", query, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *daemonClient) InvokeCmdPaletteCommand(
	ctx context.Context,
	commandID string,
	request contract.CmdPaletteInvokeRequest,
) (contract.CmdPaletteInvokeResult, error) {
	var response contract.CmdPaletteInvokeResult
	path := "/api/cmd-palette/commands/" + url.PathEscape(strings.TrimSpace(commandID)) + "/invoke"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return contract.CmdPaletteInvokeResult{}, err
	}
	return response, nil
}

func (c *daemonClient) GetCmdPalettePersonalization(
	ctx context.Context,
	workspace string,
) (contract.CmdPalettePersonalizationResponse, error) {
	query := url.Values{cmdPaletteWorkspaceFlag: {workspace}}
	var response contract.CmdPalettePersonalizationResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/cmd-palette/personalization", query, nil, &response); err != nil {
		return contract.CmdPalettePersonalizationResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) ResetCmdPalettePersonalization(
	ctx context.Context,
	workspace string,
) (contract.CmdPalettePersonalizationResetResponse, error) {
	query := url.Values{cmdPaletteWorkspaceFlag: {workspace}}
	var response contract.CmdPalettePersonalizationResetResponse
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		"/api/cmd-palette/personalization",
		query,
		nil,
		&response,
	); err != nil {
		return contract.CmdPalettePersonalizationResetResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) GetPendingToolApproval(
	ctx context.Context,
	approvalID string,
) (contract.ToolApprovalStatusResponse, error) {
	var response contract.ToolApprovalStatusResponse
	path := "/api/tools/approvals/" + url.PathEscape(strings.TrimSpace(approvalID))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.ToolApprovalStatusResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) CancelPendingToolApproval(
	ctx context.Context,
	approvalID string,
) (contract.ToolApprovalStatusResponse, error) {
	var response contract.ToolApprovalStatusResponse
	path := "/api/tools/approvals/" + url.PathEscape(strings.TrimSpace(approvalID)) + "/cancel"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return contract.ToolApprovalStatusResponse{}, err
	}
	return response, nil
}
