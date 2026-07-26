package spec

import "github.com/compozy/agh/internal/api/contract"

const (
	modelCatalogForbiddenDescription                 = "Forbidden"
	modelCatalogInternalServerErrorDescription       = "Internal server error"
	modelCatalogInvalidModelCatalogFilterDescription = "Invalid model catalog filter"
	modelCatalogModelCatalogUnavailableDescription   = "Model catalog unavailable"
	specOpenAIKey                                    = "openai"
	modelCatalogProvidersKey                         = "providers"
)

func modelCatalogOperations() []OperationSpec {
	operations := []OperationSpec{openAIModelCatalogOperation()}
	return append(operations, nativeModelCatalogOperations()...)
}

func openAIModelCatalogOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/openai/v1/models",
		OperationID: "listOpenAIModels",
		Summary:     "List provider models using the OpenAI-compatible model shape",
		Tags:        []string{specOpenAIKey},
		Transports:  []Transport{TransportHTTP},
		Parameters: []ParameterSpec{
			queryParam("provider_id", "Filter by AGH provider id", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.OpenAIModelListResponse{}},
			{
				Status:      400,
				Description: modelCatalogInvalidModelCatalogFilterDescription,
				Body:        contract.OpenAIErrorResponse{},
			},
			{Status: 401, Description: "Unauthorized", Body: contract.OpenAIErrorResponse{}},
			{Status: 403, Description: modelCatalogForbiddenDescription, Body: contract.OpenAIErrorResponse{}},
			{
				Status:      503,
				Description: modelCatalogModelCatalogUnavailableDescription,
				Body:        contract.OpenAIErrorResponse{},
			},
			{
				Status:      500,
				Description: modelCatalogInternalServerErrorDescription,
				Body:        contract.OpenAIErrorResponse{},
			},
		},
	}
}

func nativeModelCatalogOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/model-catalog/models",
			OperationID: "listProviderModels",
			Summary:     "List provider model catalog entries across providers",
			Tags:        []string{modelCatalogProvidersKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  modelCatalogListParameters(false),
			Responses:   modelCatalogListResponses(),
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/model-catalog/providers/{provider_id}/models",
			OperationID: "listProviderModelsByProvider",
			Summary:     "List provider model catalog entries for one provider",
			Tags:        []string{modelCatalogProvidersKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  modelCatalogListParameters(true),
			Responses:   modelCatalogListResponses(),
		},
		{
			Method:              httpMethodPost,
			Path:                "/api/model-catalog/models/refresh",
			OperationID:         "refreshProviderModels",
			Summary:             "Refresh provider model catalog sources across providers",
			Tags:                []string{modelCatalogProvidersKey},
			Transports:          []Transport{TransportHTTP, TransportUDS},
			RequestBody:         contract.ProviderModelRefreshRequest{},
			RequestBodyOptional: true,
			Responses:           modelCatalogRefreshResponses(),
		},
		{
			Method:              httpMethodPost,
			Path:                "/api/model-catalog/providers/{provider_id}/models/refresh",
			OperationID:         "refreshProviderModelsByProvider",
			Summary:             "Refresh provider model catalog sources for one provider",
			Tags:                []string{modelCatalogProvidersKey},
			Transports:          []Transport{TransportHTTP, TransportUDS},
			Parameters:          []ParameterSpec{pathParam("provider_id", "AGH provider id")},
			RequestBody:         contract.ProviderModelRefreshRequest{},
			RequestBodyOptional: true,
			Responses:           modelCatalogRefreshResponses(),
		},
		modelCatalogCurationOperation(),
		{
			Method:      httpMethodGet,
			Path:        "/api/model-catalog/sources/status",
			OperationID: "getProviderModelStatus",
			Summary:     "List provider model catalog source status across providers",
			Tags:        []string{modelCatalogProvidersKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Responses:   modelCatalogStatusResponses(),
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/model-catalog/providers/{provider_id}/models/status",
			OperationID: "getProviderModelStatusByProvider",
			Summary:     "List provider model catalog source status for one provider",
			Tags:        []string{modelCatalogProvidersKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("provider_id", "AGH provider id")},
			Responses:   modelCatalogStatusResponses(),
		},
	}
}

func modelCatalogCurationOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/model-catalog/providers/{provider_id}/models/curate",
		OperationID: "curateProviderModel",
		Summary:     "Curate one provider model through the live settings lifecycle",
		Tags:        []string{modelCatalogProvidersKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("provider_id", "AGH provider id"),
		},
		RequestBody: contract.ProviderModelCurationRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ProviderModelCurationResponse{}},
			{Status: 400, Description: "Invalid curation payload", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Unknown model or unsupported effort", Body: contract.ErrorPayload{}},
			{Status: 503, Description: modelCatalogModelCatalogUnavailableDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: modelCatalogInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func modelCatalogListParameters(providerPath bool) []ParameterSpec {
	parameters := make([]ParameterSpec, 0, 5)
	if providerPath {
		parameters = append(parameters, pathParam("provider_id", "AGH provider id"))
	} else {
		parameters = append(parameters, queryParam("provider_id", "Filter by AGH provider id", false))
	}
	parameters = append(
		parameters,
		enumQueryParam("view", "Catalog view; defaults to curated", []string{"curated", specAllKey}),
		queryParam("source_id", "Filter by catalog source id", false),
		boolQueryParam("refresh", "Refresh sources before listing models"),
		boolQueryParam("include_stale", "Include stale source rows in the merged projection"),
	)
	return parameters
}

func modelCatalogListResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "OK", Body: contract.ProviderModelListResponse{}},
		{Status: 400, Description: modelCatalogInvalidModelCatalogFilterDescription, Body: contract.ErrorPayload{}},
		{Status: 403, Description: modelCatalogForbiddenDescription, Body: contract.ErrorPayload{}},
		{Status: 503, Description: modelCatalogModelCatalogUnavailableDescription, Body: contract.ErrorPayload{}},
		{Status: 500, Description: modelCatalogInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}

func modelCatalogRefreshResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "OK", Body: contract.ProviderModelRefreshResponse{}},
		{Status: 400, Description: "Invalid model catalog refresh request", Body: contract.ErrorPayload{}},
		{Status: 403, Description: modelCatalogForbiddenDescription, Body: contract.ErrorPayload{}},
		{Status: 503, Description: "Model catalog refresh unavailable", Body: contract.ProviderModelRefreshResponse{}},
		{Status: 500, Description: modelCatalogInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}

func modelCatalogStatusResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "OK", Body: contract.ProviderModelStatusResponse{}},
		{Status: 400, Description: modelCatalogInvalidModelCatalogFilterDescription, Body: contract.ErrorPayload{}},
		{Status: 403, Description: modelCatalogForbiddenDescription, Body: contract.ErrorPayload{}},
		{Status: 503, Description: modelCatalogModelCatalogUnavailableDescription, Body: contract.ErrorPayload{}},
		{Status: 500, Description: modelCatalogInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}
