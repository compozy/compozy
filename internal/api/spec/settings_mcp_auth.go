package spec

import "github.com/compozy/agh/internal/api/contract"

const specContentTypeHTML = "text/html"

func settingsMCPAuthOperations() []OperationSpec {
	basePath := "/api/settings/mcp-servers/{name}/auth/"
	targetParameters := []ParameterSpec{
		pathParam("name", "Configured MCP server name"),
		{
			Name: specScopeKey, In: "query", Description: "Exact MCP settings scope",
			Required: true, Kind: "string", Enum: []string{specGlobalKey, specWorkspaceKey},
		},
		queryParam("workspace_id", "Required when scope is workspace", false),
	}
	responses := func(success any) []ResponseSpec {
		return []ResponseSpec{
			{Status: 200, Description: "OK", Body: success},
			{Status: 400, Description: "Invalid MCP OAuth request or session", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "MCP server or workspace not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: "MCP OAuth runtime is unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		}
	}
	return []OperationSpec{
		{
			Method: httpMethodPost, Path: basePath + "begin", OperationID: "beginSettingsMCPAuth",
			Summary: "Begin daemon-mediated OAuth for one MCP server", Tags: []string{specSettingsKey},
			Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: targetParameters,
			RequestBody: contract.SettingsMCPAuthBeginRequest{},
			Responses:   responses(contract.SettingsMCPAuthBeginResponse{}),
		},
		{
			Method: httpMethodPost, Path: basePath + "exchange", OperationID: "exchangeSettingsMCPAuth",
			Summary: "Complete daemon-mediated OAuth for one MCP server", Tags: []string{specSettingsKey},
			Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: targetParameters,
			RequestBody: contract.SettingsMCPAuthExchangeRequest{},
			Responses:   responses(contract.SettingsMCPAuthStatusPayload{}),
		},
		{
			Method: httpMethodPost, Path: basePath + "logout", OperationID: "logoutSettingsMCPAuth",
			Summary: "Log out one scoped MCP server", Tags: []string{specSettingsKey},
			Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: targetParameters,
			Responses: responses(contract.SettingsMCPAuthStatusPayload{}),
		},
		{
			Method: httpMethodGet, Path: "/api/mcp/oauth/callback", OperationID: "completeMCPAuthCallback",
			Summary: "Complete a loopback MCP OAuth browser redirect", Tags: []string{specSettingsKey},
			Transports: []Transport{TransportHTTP},
			Parameters: []ParameterSpec{
				queryParam("code", "Authorization code returned by the provider", false),
				queryParam("state", "Single-use OAuth state", true),
				queryParam("error", "OAuth error returned by the provider", false),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Authorization completed", Body: "", ContentType: specContentTypeHTML},
				{Status: 400, Description: "Authorization failed", Body: "", ContentType: specContentTypeHTML},
				{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
				{
					Status: 503, Description: "MCP OAuth callback runtime is unavailable",
					Body: "", ContentType: specContentTypeHTML,
				},
			},
		},
	}
}
