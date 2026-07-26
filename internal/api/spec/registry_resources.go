package spec

import "github.com/compozy/agh/internal/api/contract"

func registryResourceOperations() []OperationSpec {
	return []OperationSpec{
		listResourcesOperationSpec(),
		listResourcesByKindOperationSpec(),
		getResourceOperationSpec(),
		putResourceOperationSpec(),
		deleteResourceOperationSpec(),
	}
}
func listResourcesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/resources",
		OperationID: "listResources",
		Summary:     "List desired-state resources on the local operator control plane",
		Tags:        []string{specResourcesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("kind", "Filter by resource kind", false),
			enumQueryParam("scope_kind", "Filter by resource scope kind", resourceScopeKindValues()),
			queryParam("scope_id", "Filter by workspace scope id", false),
			queryParam("owner_kind", "Filter by stamped owner kind", false),
			queryParam("owner_id", "Filter by stamped owner id", false),
			queryParam("source_kind", "Filter by stamped source kind", false),
			queryParam("source_id", "Filter by stamped source id", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ResourcesResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid resource filter", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listResourcesByKindOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/resources/{kind}",
		OperationID: "listResourcesByKind",
		Summary:     "List one desired-state resource kind on the local operator control plane",
		Tags:        []string{specResourcesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("kind", "Resource kind"),
			enumQueryParam("scope_kind", "Filter by resource scope kind", resourceScopeKindValues()),
			queryParam("scope_id", "Filter by workspace scope id", false),
			queryParam("owner_kind", "Filter by stamped owner kind", false),
			queryParam("owner_id", "Filter by stamped owner id", false),
			queryParam("source_kind", "Filter by stamped source kind", false),
			queryParam("source_id", "Filter by stamped source id", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ResourcesResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid resource filter", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getResourceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIResourcesKindIDPath,
		OperationID: "getResource",
		Summary:     "Read one desired-state resource on the local operator control plane",
		Tags:        []string{specResourcesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("kind", "Resource kind"),
			pathParam("id", "Resource id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ResourceResponse{}},
			{Status: 404, Description: "Resource not found", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid resource identifier", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func putResourceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPIResourcesKindIDPath,
		OperationID: "putResource",
		Summary:     "Create or replace one desired-state resource on the local operator control plane",
		Tags:        []string{specResourcesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("kind", "Resource kind"),
			pathParam("id", "Resource id"),
		},
		RequestBody: contract.PutResourceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Updated", Body: contract.ResourceResponse{}},
			{Status: 201, Description: specCreatedDescription, Body: contract.ResourceResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflict", Body: contract.ErrorPayload{}},
			{Status: 413, Description: specPayloadTooLargeDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid resource payload", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specRateLimitedDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteResourceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIResourcesKindIDPath,
		OperationID: "deleteResource",
		Summary:     "Delete one desired-state resource on the local operator control plane",
		Tags:        []string{specResourcesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("kind", "Resource kind"),
			pathParam("id", "Resource id"),
		},
		RequestBody: contract.DeleteResourceRequest{},
		Responses: []ResponseSpec{
			{Status: 204, Description: "Deleted"},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Resource not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid delete request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specRateLimitedDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
