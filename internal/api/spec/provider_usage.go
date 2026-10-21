package spec

import "github.com/compozy/compozy/internal/api/contract"

func providerUsageOperations() []OperationSpec {
	list := OperationSpec{
		Method: httpMethodGet, Path: "/api/provider-usage", OperationID: "listProviderUsage",
		Summary: "Read cached provider usage (experimental)", Stability: "experimental",
		AppAccess: AppAccessAllowed, Tags: []string{specProvidersKey},
		Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{queryParam("profile", "Profile name; defaults to the caller session profile or default", false)},
		Responses:  providerUsageResponses(),
	}
	provider := list
	provider.Path = "/api/provider-usage/providers/{provider_id}"
	provider.OperationID = "getProviderUsage"
	provider.Parameters = append([]ParameterSpec{pathParam("provider_id", "Canonical provider ID")}, list.Parameters...)
	refresh := list
	refresh.Method, refresh.Path, refresh.OperationID = httpMethodPost, "/api/provider-usage/refresh", "refreshProviderUsage"
	refresh.Summary = "Refresh provider usage (experimental)"
	refresh.AppAccess = AppAccessDenied
	refresh.Parameters = nil
	refresh.RequestBody, refresh.RequestBodyOptional = contract.ProviderUsageRefreshRequest{}, true
	stream := list
	stream.Path, stream.OperationID = "/api/provider-usage/stream", "streamProviderUsage"
	stream.Summary = "Stream provider usage invalidations (experimental)"
	stream.Responses = providerUsageResponses()
	stream.Responses[0] = ResponseSpec{Status: 200, Description: "ready and provider_usage_changed revision events", ContentType: specContentTypeEventStream, Body: contract.ProviderUsageStreamEvent{}}
	return []OperationSpec{list, provider, refresh, stream}
}

func providerUsageResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "Profile-scoped usage envelope", Body: contract.ProviderUsageEnvelope{}},
		{Status: 400, Description: "Invalid request", Body: contract.ErrorPayload{}},
		{Status: 403, Description: "App token route forbidden", Body: contract.ErrorPayload{}},
		{Status: 404, Description: "Profile or provider not found", Bodies: overlappingResponseBodiesOf(responseBodyOf[contract.ErrorPayload](), responseBodyOf[contract.ProfileErrorPayload]())},
		{Status: 409, Description: "Usage disabled or session profile conflict", Bodies: overlappingResponseBodiesOf(responseBodyOf[contract.ErrorPayload](), responseBodyOf[contract.ProfileErrorPayload]())},
		{Status: 503, Description: "Usage service unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: "Internal server error", Body: contract.ErrorPayload{}},
	}
}
