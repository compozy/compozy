package spec

import "github.com/compozy/agh/internal/api/contract"

func sessionCatalogOperations() []OperationSpec {
	return []OperationSpec{
		sessionCatalogListOperation(),
		sessionCatalogStreamOperation(),
	}
}

func sessionCatalogListOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specSessionsPath,
		OperationID: "listSessions",
		Summary:     "List sessions",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam(specWorkspaceKey, "Workspace id or path", false),
			boolQueryParam("include_health", "Include metadata-only health for returned sessions"),
			enumQueryParam(
				"state",
				"Filter by exact session state",
				[]string{"starting", "active", "stopping", "stopped"},
			),
			enumQueryParam("type", "Filter by exact session type", sessionCatalogTypeValues()),
			queryParam("agent", "Filter by exact agent definition name", false),
			queryParam("q", "Search session id, name, agent, provider, or channel", false),
			boolQueryParam("resumable", "Only list sessions eligible for explicit attach"),
			enumQueryParam("sort", "Stable session ordering", []string{"recent", "last_activity"}),
			queryParam("cursor", "Opaque next_cursor from the previous page", false),
			intQueryParam("limit", "Sessions per page (1-100)"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionCatalogResponse{}},
			{Status: 400, Description: "Invalid session list query or cursor", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 410, Description: workspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: "Paged session catalog or workspace resolver is unavailable",
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func sessionCatalogTypeValues() []string {
	return []string{"user", "system", "coordinator", "spawned"}
}

func sessionCatalogStreamOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/sessions/catalog-stream",
		OperationID: "streamSessionCatalog",
		Summary:     "Stream session catalog changes across workspaces",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Workspace-identified session catalog event stream",
				Body:        contract.SessionCatalogEventPayload{},
				ContentType: specContentTypeEventStream,
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Session catalog stream is unavailable", Body: contract.ErrorPayload{}},
		},
	}
}
