package spec

import "github.com/compozy/compozy/internal/api/contract"

const specContentTypeHTML = "text/html"

func settingsMCPAuthOperations() []OperationSpec {
	basePath := "/api/settings/mcp-servers/{name}/auth/"
	targetParameters := settingsMCPAuthParameters()
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        basePath + "status",
			AppAccess:   AppAccessDenied,
			OperationID: "getSettingsMCPAuthStatus",
			Summary:     "Get redacted daemon-mediated OAuth state for one MCP server", Tags: []string{specSettingsKey},
			Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: targetParameters,
			Responses: settingsMCPAuthResponses(contract.SettingsMCPAuthStatusPayload{}),
		},
		{
			Method:      httpMethodPost,
			Path:        basePath + "begin",
			AppAccess:   AppAccessDenied,
			OperationID: "beginSettingsMCPAuth",
			Summary:     "Begin daemon-mediated OAuth for one MCP server",
			Tags:        []string{specSettingsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  targetParameters,
			RequestBody: contract.SettingsMCPAuthBeginRequest{},
			Responses:   settingsMCPAuthResponses(contract.SettingsMCPAuthBeginResponse{}),
		},
		{
			Method:      httpMethodPost,
			Path:        basePath + "exchange",
			AppAccess:   AppAccessDenied,
			OperationID: "exchangeSettingsMCPAuth",
			Summary:     "Complete daemon-mediated OAuth for one MCP server", Tags: []string{specSettingsKey},
			Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: targetParameters,
			RequestBody: contract.SettingsMCPAuthExchangeRequest{},
			Responses:   settingsMCPAuthResponses(contract.SettingsMCPAuthStatusPayload{}),
		},
		{
			Method:      httpMethodPost,
			Path:        basePath + "logout",
			AppAccess:   AppAccessDenied,
			OperationID: "logoutSettingsMCPAuth",
			Summary:     "Log out one scoped MCP server",
			Tags:        []string{specSettingsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  targetParameters,
			Responses:   settingsMCPAuthResponses(contract.SettingsMCPAuthStatusPayload{}),
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/mcp/oauth/callback",
			AppAccess:   AppAccessDenied,
			OperationID: "completeMCPAuthCallback",
			Summary:     "Complete a loopback MCP OAuth browser redirect", Tags: []string{specSettingsKey},
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

func settingsMCPAuthParameters() []ParameterSpec {
	return []ParameterSpec{
		pathParam("name", "Configured MCP server name"),
		{
			Name: specScopeKey, In: specParameterInQuery, Description: "Exact MCP settings scope",
			Required: true, Kind: specStringKey, Enum: settingsLayeredScopeValues(),
		},
		queryParam("workspace_id", "Required when scope is workspace", false),
		queryParam("profile", "Required when scope is profile", false),
		queryParam(
			"owner",
			"Definition owner: manual or extension:<name>; omitted owner selects manual",
			false,
		),
	}
}

func settingsMCPAuthResponses(success any) []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "OK", Body: success},
		{Status: 400, Description: "Invalid MCP OAuth request or session", Body: contract.ErrorPayload{}},
		{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
		{Status: 404, Description: settingsMCPNotFoundDescription, Body: contract.ErrorPayload{}},
		{Status: 503, Description: "MCP OAuth runtime is unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}
