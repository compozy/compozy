package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSettingsWindowManagerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsWindowManagerPath,
		OperationID: "getSettingsWindowManager",
		Summary:     "Read the window-manager settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Select the settings scope", settingsWorkspaceScopeValues()),
			queryParam("workspace_id", "Select the workspace id for workspace scope", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsWindowManagerResponse{}},
			{Status: 400, Description: specInvalidSettingsScopeDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateSettingsWindowManagerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsWindowManagerPath,
		OperationID: "updateSettingsWindowManager",
		Summary:     "Update the window-manager settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Select the settings scope", settingsWorkspaceScopeValues()),
			queryParam("workspace_id", "Select the workspace id for workspace scope", false),
		},
		RequestBody: contract.UpdateSettingsWindowManagerRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsWindowManagerResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status: 409, Description: specConflictingSettingsChangeDescription,
				Body: contract.SettingsWindowManagerMutationError{},
			},
			{
				Status: 422, Description: specInvalidSettingsPayloadDescription,
				Body: contract.SettingsWindowManagerMutationError{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
