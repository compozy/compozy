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

func importRuntimeMCPServerOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: "/api/settings/runtime-mcp-servers/{runtime}/{name}/import", AppAccess: AppAccessDenied,
		OperationID: "importRuntimeMCPServer", Summary: "Copy a runtime MCP server into CompozyOS", Tags: []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("runtime", "Source runtime"), pathParam("name", "URL-escaped runtime server name")},
		RequestBody: contract.RuntimeMCPImportRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: "Copied, possibly with partial cleanup warnings", Body: contract.RuntimeMCPImportResponse{}},
			{Status: 400, Description: "Invalid import request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Runtime server or workspace not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Target name already exists", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Runtime source, transport or definition cannot be copied", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
