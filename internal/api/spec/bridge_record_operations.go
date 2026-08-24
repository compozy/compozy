package spec

import "github.com/compozy/compozy/internal/api/contract"

func bridgeRecordOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        specBridgeRecordPath,
			OperationID: "getBridge",
			Summary:     "Get one bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(
				pathParam("id", "Bridge instance id"),
			),
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPatch,
			Path:        specBridgeRecordPath,
			OperationID: "updateBridge",
			Summary:     "Update mutable bridge instance fields",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("id", "Bridge instance id"),
			),
			RequestBody: contract.UpdateBridgeRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeResponse{}},
				{Status: 400, Description: "Invalid bridge update", Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Bridge instance or workspace not found", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
