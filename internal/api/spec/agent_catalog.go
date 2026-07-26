package spec

import "github.com/compozy/agh/internal/api/contract"

func agentCatalogOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/agents/catalog",
			OperationID: "listAgentCatalog",
			Summary:     "List a filtered workspace agent fleet with exact session metrics",
			Tags:        []string{specAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam(specWorkspaceKey, "Workspace id, name, or path", true),
				queryParam("name", "Exact agent name", false),
				queryParam("q", "Case-insensitive agent name or category search", false),
				queryParam("category", "Exact category path joined by slash separators", false),
				enumQueryParam("status", "Agent session status", []string{"active", "idle"}),
				queryParam("cursor", "Opaque cursor bound to workspace and filters", false),
				intQueryParam("limit", "Agents per page (1-100; default 50)"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.AgentCatalogResponse{}},
				{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 410, Description: specWorkspaceRootMissingDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Workspace resolver unavailable", Body: contract.ErrorPayload{}},
			},
		},
	}
}
