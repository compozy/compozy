package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

type MCPApprovalClient interface {
	SetSettingsMCPApproval(context.Context, string, contract.SettingsMCPApprovalRequest, bool) (contract.SettingsMCPApprovalResponse, error)
}

func (c *daemonClient) SetSettingsMCPApproval(ctx context.Context, name string, body contract.SettingsMCPApprovalRequest, revoke bool) (contract.SettingsMCPApprovalResponse, error) {
	path := "/api/settings/mcp-servers/" + url.PathEscape(name) + "/approval"
	var result contract.SettingsMCPApprovalResponse
	if revoke {
		err := c.doJSON(ctx, http.MethodDelete, path, url.Values{"workspace_id": {body.WorkspaceID}, "profile": {body.Profile}}, nil, nil)
		return result, err
	}
	err := c.doJSON(ctx, http.MethodPut, path, nil, body, &result)
	return result, err
}
