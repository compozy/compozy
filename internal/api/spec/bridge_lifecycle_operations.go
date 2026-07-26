package spec

import "github.com/compozy/agh/internal/api/contract"

func bridgeLifecycleOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/enable",
			OperationID: "enableBridge",
			Summary:     "Enable a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: specInvalidBridgeStateTransitionDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/disable",
			OperationID: "disableBridge",
			Summary:     "Disable a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: specInvalidBridgeStateTransitionDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/restart",
			OperationID: "restartBridge",
			Summary:     "Restart a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Bridge instance id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: specInvalidBridgeStateTransitionDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
