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
	GetPendingToolApproval(context.Context, string) (contract.ToolApprovalStatusResponse, error)
	CancelPendingToolApproval(context.Context, string) (contract.ToolApprovalStatusResponse, error)
}

var _ CmdPaletteClient = (*daemonClient)(nil)

func (c *daemonClient) ListCmdPaletteCommands(
	ctx context.Context,
	workspace string,
	clientID string,
) (contract.CmdPaletteCommandsResponse, error) {
	query := url.Values{"workspace": {workspace}}
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
	query := url.Values{"workspace": {workspace}}
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
