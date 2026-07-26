package spec

import "github.com/compozy/agh/internal/api/contract"

func bridgeCatalogOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/bridges",
			OperationID: "listBridges",
			Summary:     "List persisted bridge instances",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  bridgeCatalogQueryParams(),
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgesResponse{}},
				{Status: 400, Description: "Invalid bridge list filter", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 410, Description: "Workspace root is unavailable", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/bridges",
			OperationID: "createBridge",
			Summary:     "Create a bridge instance",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			RequestBody: contract.CreateBridgeRequest{},
			Responses: []ResponseSpec{
				{Status: 201, Description: specCreatedDescription, Body: contract.BridgeResponse{}},
				{Status: 400, Description: "Invalid bridge request", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/bridges/providers",
			OperationID: "listBridgeProviders",
			Summary:     "List installed bridge-capable providers",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.BridgeProvidersResponse{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/bridges/health/stream",
			OperationID: "streamBridgeHealth",
			Summary:     "Stream bridge health snapshots",
			Tags:        []string{specBridgesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam("bridge_ids", "Comma-separated bridge ids from the current catalog page; maximum 200", true),
				enumQueryParam(
					specScopeKey,
					"Filter by bridge scope",
					[]string{specAllKey, specGlobalKey, specWorkspaceKey},
				),
				queryParam("workspace_id", "Filter by active workspace id", false),
				queryParam(specWorkspaceKey, "Filter by workspace id, name, or path", false),
			},
			Responses: []ResponseSpec{
				{
					Status:      200,
					Description: "Bridge health snapshot stream",
					Body:        contract.BridgeHealthStreamPayload{},
					ContentType: specContentTypeEventStream,
				},
				{Status: 400, Description: "Invalid bridge list filter", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 410, Description: "Workspace root is unavailable", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specBridgeServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
