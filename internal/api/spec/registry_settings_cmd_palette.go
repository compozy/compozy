package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSettingsCmdPaletteOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPISettingsCmdPalettePath,
		OperationID: "getSettingsCmdPalette", Summary: "Read command-palette settings",
		Tags: []string{specSettingsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Select the settings scope", settingsWorkspaceScopeValues()),
			queryParam("workspace_id", "Select the workspace id for workspace scope", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsCmdPaletteResponse{}},
			{Status: 400, Description: specInvalidSettingsScopeDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateSettingsCmdPaletteOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPatch, Path: specAPISettingsCmdPalettePath,
		OperationID: "updateSettingsCmdPalette", Summary: "Update command-palette settings",
		Tags: []string{specSettingsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Select the settings scope", settingsWorkspaceScopeValues()),
			queryParam("workspace_id", "Select the workspace id for workspace scope", false),
		},
		RequestBody: contract.UpdateSettingsCmdPaletteRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsCmdPaletteResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
