package spec

import "github.com/compozy/agh/internal/api/contract"

func bridgeSecretOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/bridges/{id}/secret-bindings",
			OperationID: "listBridgeSecretBindings",
			Summary:     "List persisted secret bindings for a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeSecretBindingsResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPut,
			Path:        "/api/bridges/{id}/secret-bindings/{binding_name}",
			OperationID: "putBridgeSecretBinding",
			Summary:     "Create or update one bridge secret binding",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
				pathParam("binding_name", "Bridge provider secret slot name"),
			},
			RequestBody: contract.PutBridgeSecretBindingRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeSecretBindingResponse{}},
				{Status: 400, Description: "Invalid bridge secret binding request", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Bridge secret binding conflict", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodDelete,
			Path:        "/api/bridges/{id}/secret-bindings/{binding_name}",
			OperationID: "deleteBridgeSecretBinding",
			Summary:     "Delete one bridge secret binding",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
				pathParam("binding_name", "Bridge provider secret slot name"),
			},
			Responses: []ResponseSpec{
				{Status: 204, Description: specNoContentDescription},
				{
					Status:      404,
					Description: "Bridge instance or secret binding not found",
					Body:        contract.ErrorPayload{},
				},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
