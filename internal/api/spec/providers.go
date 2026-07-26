package spec

import "github.com/compozy/agh/internal/api/contract"

func providerOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/providers",
			OperationID: "listProviders",
			Summary:     "List providers and declared auth status",
			Tags:        []string{specProvidersKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ProviderListResponse{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/providers/{provider_id}",
			OperationID: "getProvider",
			Summary:     "Get one provider and declared auth status",
			Tags:        []string{specProvidersKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("provider_id", "AGH provider id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ProviderSummaryPayload{}},
				{Status: 404, Description: specProviderNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/providers/{provider_id}/auth/probe",
			OperationID: "probeProviderAuth",
			Summary:     "Run a non-interactive provider auth status probe",
			Tags:        []string{specProvidersKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("provider_id", "AGH provider id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ProviderAuthProbeResponse{}},
				{Status: 404, Description: specProviderNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 422, Description: "Provider auth probe unavailable", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
