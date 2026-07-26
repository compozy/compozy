package spec

import "github.com/compozy/agh/internal/api/contract"

func registrySupportOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodPost,
		Path:        "/api/support/bundles",
		OperationID: "createSupportBundle",
		Summary:     "Create a support bundle",
		Tags:        []string{specSupportKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateSupportBundleRequest{},
		Responses: []ResponseSpec{
			{Status: 202, Description: specAcceptedDescription, Body: contract.SupportBundleOperationResponse{}},
			{Status: 400, Description: "Invalid support bundle request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specSupportBundleServiceIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodGet,
			Path:        "/api/support/bundles/{operation_id}",
			OperationID: "getSupportBundle",
			Summary:     "Get support bundle operation status",
			Tags:        []string{specSupportKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("operation_id", "Support bundle operation id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.SupportBundleOperationResponse{}},
				{Status: 404, Description: "Support bundle operation not found", Body: contract.ErrorPayload{}},
				{
					Status:      503,
					Description: specSupportBundleServiceIsNotConfiguredDescription,
					Body:        contract.ErrorPayload{},
				},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/support/bundles/{operation_id}/download",
			OperationID: "downloadSupportBundle",
			Summary:     "Download a completed support bundle archive",
			Tags:        []string{specSupportKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("operation_id", "Support bundle operation id"),
			},
			Responses: []ResponseSpec{
				{
					Status:      200,
					Description: "Support bundle archive",
					Body:        binaryResponse{},
					ContentType: "application/gzip",
				},
				{Status: 404, Description: "Support bundle operation not found", Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Support bundle operation is not ready", Body: contract.ErrorPayload{}},
				{
					Status:      503,
					Description: specSupportBundleServiceIsNotConfiguredDescription,
					Body:        contract.ErrorPayload{},
				},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		}}
}
