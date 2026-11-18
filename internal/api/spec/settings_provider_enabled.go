package spec

import "github.com/compozy/compozy/internal/api/contract"

func setSettingsProviderEnabledOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsProvidersNamePath + "/enabled",
		AppAccess:   AppAccessDenied,
		OperationID: "setSettingsProviderEnabled",
		Summary:     "Enable or disable one provider",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Provider name"),
		},
		RequestBody: contract.SetSettingsProviderEnabledRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsProviderEnabledResponse{}},
			{Status: 400, Description: "Invalid enabled payload", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specProviderNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      422,
				Description: "Disabling the default provider is rejected",
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
