package spec

import "github.com/compozy/agh/internal/api/contract"

func bridgeControlOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/verify",
			OperationID: "verifyBridge",
			Summary:     "Run live provider checks without changing bridge lifecycle state",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("id", "Bridge instance id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeVerifyResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/send-test",
			OperationID: "sendBridgeTest",
			Summary:     "Send one real message through the bridge provider",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("id", "Bridge instance id")},
			RequestBody: contract.BridgeSendTestRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeSendTestResponse{}},
				{Status: 400, Description: "Invalid send-test request", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Bridge instance is unavailable", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges/{id}/webhook/register",
			OperationID: "registerBridgeWebhook",
			Summary:     "Register the configured provider webhook",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("id", "Bridge instance id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeWebhookRegistrationResponse{}},
				{Status: 404, Description: specBridgeInstanceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
