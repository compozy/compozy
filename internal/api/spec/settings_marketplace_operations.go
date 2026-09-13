package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSettingsMarketplaceOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/marketplace",
		OperationID: "getSettingsMarketplace",
		Summary:     "Read global marketplace catalog settings",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Catalog settings", Body: contract.SettingsMarketplaceResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateSettingsMarketplaceOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        "/api/settings/marketplace",
		OperationID: "updateSettingsMarketplace",
		Summary:     "Apply global marketplace catalog URL, TTL, and timeout",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsMarketplaceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Applied settings", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
