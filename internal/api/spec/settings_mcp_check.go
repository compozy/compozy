package spec

import "github.com/compozy/compozy/internal/api/contract"

func checkSettingsMCPOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: "/api/settings/mcp-servers/{name}/check",
		AppAccess: AppAccessDenied, OperationID: "checkSettingsMCPServer",
		Summary: "Retry one scoped MCP server runtime probe", Tags: []string{specSettingsKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: settingsMCPAuthParameters(),
		Responses: mcpCheckResponses(contract.SettingsMCPServerItemPayload{}),
	}
}

func mcpCheckResponses(success any) []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "MCP check result", Body: success},
		{Status: 400, Description: "Invalid MCP check request or disabled source", Body: contract.ErrorPayload{}},
		{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
		{Status: 404, Description: settingsMCPNotFoundDescription, Body: contract.ErrorPayload{}},
		{Status: 499, Description: "MCP check canceled by client", Body: contract.ErrorPayload{}},
		{Status: 503, Description: "MCP check runtime unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}
