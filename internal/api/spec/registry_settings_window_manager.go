package spec

import "github.com/compozy/agh/internal/api/contract"

func getSettingsWindowManagerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsWindowManagerPath,
		OperationID: "getSettingsWindowManager",
		Summary:     "Read the window-manager settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsWindowManagerResponse{}},
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
		RequestBody: contract.UpdateSettingsWindowManagerRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
