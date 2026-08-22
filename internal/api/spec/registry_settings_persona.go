package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSettingsPersonaOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsPersonaPath,
		OperationID: "getSettingsPersona",
		Summary:     "Read profile-layerable persona defaults",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsLayeredParameters(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsPersonaResponse{}},
			{Status: 400, Description: specInvalidSettingsScopeDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateSettingsPersonaOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsPersonaPath,
		OperationID: "updateSettingsPersona",
		Summary:     "Update profile-layerable persona defaults",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsLayeredParameters(),
		RequestBody: contract.UpdateSettingsPersonaRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func settingsLayeredParameters() []ParameterSpec {
	return []ParameterSpec{
		enumQueryParam(specScopeKey, "Select the settings scope", settingsLayeredScopeValues()),
		queryParam("workspace_id", "Select the workspace context", false),
		queryParam("profile", "Select the profile layer", false),
	}
}
