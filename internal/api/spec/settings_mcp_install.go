package spec

import "github.com/compozy/agh/internal/api/contract"

func settingsMCPInstallOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/settings/mcp-servers/install",
		OperationID: "installSettingsMCPServer",
		Summary:     "Install one curated MCP server through settings",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.InstallSettingsMCPServerRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.InstallSettingsMCPServerResponse{}},
			{Status: 400, Description: "Invalid MCP catalog install request", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "MCP catalog entry or workspace not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflicting MCP settings target", Body: contract.ErrorPayload{}},
			{
				Status:      422,
				Description: "Catalog entry cannot produce a valid MCP server",
				Body:        contract.ErrorPayload{},
			},
			{Status: 503, Description: "Marketplace catalog is not configured", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
