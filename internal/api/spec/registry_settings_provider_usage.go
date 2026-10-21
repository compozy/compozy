package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSettingsProviderUsageOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/provider-usage",
		AppAccess:   AppAccessDenied,
		OperationID: "getSettingsProviderUsage",
		Summary:     "Read the provider usage settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsProviderUsageResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateSettingsProviderUsageOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        "/api/settings/provider-usage",
		AppAccess:   AppAccessDenied,
		OperationID: "updateSettingsProviderUsage",
		Summary:     "Update the provider usage settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.SettingsProviderUsageUpdateRequest{},
		Parameters: []ParameterSpec{
			queryParam(
				contract.ProviderUsageCollectionProfileQuery,
				"Collection profile name or ID for local Keychain consent; independent of user config scope; defaults to default",
				false,
			),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{
				Status:      400,
				Description: specInvalidSettingsPayloadDescription,
				Bodies: overlappingResponseBodiesOf(
					responseBodyOf[contract.ErrorPayload](),
					responseBodyOf[contract.ProfileErrorPayload](),
				),
			},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Collection profile not found", Body: contract.ProfileErrorPayload{}},
			{
				Status:      409,
				Description: specConflictingSettingsChangeDescription,
				Bodies: overlappingResponseBodiesOf(
					responseBodyOf[contract.ErrorPayload](),
					responseBodyOf[contract.ProfileErrorPayload](),
				),
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
