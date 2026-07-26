package spec

import "github.com/compozy/agh/internal/api/contract"

func bridgeManifestOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/bridges/providers/slack/manifest",
			OperationID: "getSlackBridgeManifest",
			Summary:     "Generate a Slack app manifest for one bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam("instance", "Slack bridge instance id", true),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.SlackAppManifestResponse{}},
				{Status: 400, Description: "Invalid Slack bridge manifest request", Body: contract.ErrorPayload{}},
				{
					Status:      404,
					Description: "Slack bridge instance or provider not found",
					Body:        contract.ErrorPayload{},
				},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
