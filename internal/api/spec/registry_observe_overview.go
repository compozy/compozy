package spec

import "github.com/compozy/agh/internal/api/contract"

func getObserveOverviewOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/observe/overview",
		OperationID: "getObserveOverview",
		Summary:     "Get the observer-backed home overview",
		Tags:        []string{specObserveKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam(
				specWorkspaceKey,
				"Scope aggregates to one workspace; empty selects the global home scope",
				false,
			),
			enumQueryParam("usage_window", "Usage window in days (default 30)", []string{"7", "30", "90"}),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ObserveOverviewResponse{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid overview query", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specObserveNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
