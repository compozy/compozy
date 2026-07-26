package spec

import "github.com/compozy/agh/internal/api/contract"

func bridgeDeliveryOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/test-delivery",
			OperationID: "testBridgeDelivery",
			Summary:     "Resolve a typed outbound delivery target for a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
			},
			RequestBody: contract.BridgeTestDeliveryRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeTestDeliveryResponse{}},
				{Status: 400, Description: "Invalid delivery target request", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Bridge instance is unavailable", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
