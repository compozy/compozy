package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSettingsAttentionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsAttentionPath,
		OperationID: "getSettingsAttention",
		Summary:     "Read the operator attention settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsAttentionParameters(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsAttentionResponse{}},
			{Status: 400, Description: specInvalidSettingsScopeDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specProfileNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Profile unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateSettingsAttentionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsAttentionPath,
		OperationID: "updateSettingsAttention",
		Summary:     "Update the operator attention settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsAttentionParameters(),
		RequestBody: contract.UpdateSettingsAttentionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specProfileNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func settingsAttentionParameters() []ParameterSpec {
	return []ParameterSpec{
		enumQueryParam(
			specScopeKey,
			"Select user or profile attention mutes",
			[]string{specUserKey, specProfileKey},
		),
		queryParam(specProfileKey, "Select the profile whose workspace mutes are returned", false),
	}
}
