package spec

import "github.com/compozy/compozy/internal/api/contract"

func listRuntimeMCPServersOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: "/api/settings/runtime-mcp-servers", AppAccess: AppAccessDenied,
		OperationID: "listRuntimeMCPServers", Summary: "List runtime MCP servers from native configuration files", Tags: []string{specSettingsKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: append(settingsLayeredParameters(), ParameterSpec{Name: "runtime", In: "query", Kind: "string"}),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.RuntimeMCPServersResponse{}},
			{Status: 400, Description: "Invalid runtime or settings scope", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		}}
}
