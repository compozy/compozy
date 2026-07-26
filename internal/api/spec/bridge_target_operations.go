package spec

import "github.com/compozy/agh/internal/api/contract"

func bridgeTargetOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/bridges/{id}/routes",
			OperationID: "listBridgeRoutes",
			Summary:     "List routes owned by a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeRoutesResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/bridges/{id}/targets",
			OperationID: "listBridgeTargets",
			Summary:     "List discovered targets for a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
				queryParam("q", "Filter targets by display name, qualifier, or route", false),
				intQueryParam("limit", "Maximum targets to return"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeTargetsResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/resolve",
			OperationID: "resolveBridgeTarget",
			Summary:     "Resolve a bridge target name without sending",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
			},
			RequestBody: contract.BridgeResolveTargetRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeResolveTargetResponse{}},
				{Status: 400, Description: "Invalid target resolve request", Body: contract.ErrorPayload{}},
				{
					Status:      404,
					Description: "Bridge instance or target not found",
					Body:        contract.BridgeResolveTargetResponse{},
				},
				{
					Status:      422,
					Description: "Bridge target lookup is ambiguous",
					Body:        contract.BridgeResolveTargetResponse{},
				},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
