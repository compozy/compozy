package spec

import "github.com/compozy/compozy/internal/api/contract"

func checkSettingsMCPOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: "/api/settings/mcp-servers/{name}/check",
		AppAccess: AppAccessDenied, OperationID: "checkSettingsMCPServer",
		Summary: "Retry one scoped MCP server runtime probe", Tags: []string{specSettingsKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: settingsMCPAuthParameters(),
		Responses: settingsMCPAuthResponses(contract.SettingsMCPServerItemPayload{}),
	}
}
