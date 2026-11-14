package spec

import "github.com/compozy/compozy/internal/api/contract"

const mcpApprovalRevokedDescription = "Revoked"

func settingsMCPApprovalOperations() []OperationSpec {
	path := "/api/settings/mcp-servers/{name}/approval"
	responses := settingsMCPAuthResponses(contract.SettingsMCPApprovalResponse{})
	return []OperationSpec{
		{
			Method:      httpMethodPut,
			Path:        path,
			AppAccess:   AppAccessDenied,
			OperationID: "putSettingsMCPApproval",
			Summary:     "Approve an exact workspace MCP definition",
			Tags:        []string{specSettingsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("name", "Configured MCP server name")},
			RequestBody: contract.SettingsMCPApprovalRequest{},
			Responses:   responses,
		},
		{
			Method:      httpMethodDelete,
			Path:        path,
			AppAccess:   AppAccessDenied,
			OperationID: "deleteSettingsMCPApproval",
			Summary:     "Revoke workspace MCP consent",
			Tags:        []string{specSettingsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("name", "Configured MCP server name"),
				queryParam("workspace_id", "Workspace ID", true),
				queryParam("profile", "Profile name", false),
			},
			Responses: []ResponseSpec{
				{Status: 204, Description: mcpApprovalRevokedDescription},
				{Status: 409, Description: "Approval not required", Body: contract.ErrorPayload{}},
				{Status: 404, Description: settingsMCPNotFoundDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
