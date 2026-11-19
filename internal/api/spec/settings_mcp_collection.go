package spec

import "github.com/compozy/compozy/internal/api/contract"

func setSettingsMCPServerEnabledOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPut, Path: specAPISettingsMCPServersNamePath + "/enabled",
		AppAccess: AppAccessDenied, OperationID: "setSettingsMCPServerEnabled",
		Summary: "Turn one scoped MCP server on or off", Tags: []string{specSettingsKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: settingsMCPAuthParameters(),
		RequestBody: contract.SettingsMCPServerEnabledRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Updated server", Body: contract.SettingsMCPServerResponse{}},
			{Status: 400, Description: "Invalid MCP server owner, scope, or payload", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: settingsMCPNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "MCP server state could not be applied", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func getSettingsMCPServerOperationSpec() OperationSpec {
	parameters := []ParameterSpec{
		pathParam("name", "Logical MCP name with owner, otherwise manual or allocated runtime name"),
	}
	parameters = append(parameters, settingsLayeredParameters()...)
	parameters = append(
		parameters,
		queryParam("owner", "Select manual or extension:<name>; omitted owner selects manual servers only", false),
	)
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsMCPServersNamePath,
		AppAccess:   AppAccessDenied,
		OperationID: "getSettingsMCPServer",
		Summary:     "Get one owner-qualified MCP server", Tags: []string{specSettingsKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: parameters,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsMCPServerResponse{}},
			{Status: 400, Description: "Invalid MCP server owner or scope", Body: contract.ErrorPayload{}},
			{Status: 404, Description: settingsMCPNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
