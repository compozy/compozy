package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	agentIdentityDeniedDescription         = "Agent identity is denied"
	sessionAttentionUnavailableDescription = "Session attention is unavailable"
)

func sessionCatalogOperations() []OperationSpec {
	return []OperationSpec{
		sessionCatalogListOperation(),
		sessionCatalogStreamOperation(),
		sessionWaitOperation(),
		sessionPromptCancelOperation(),
		sessionPresenceOperation(),
		sessionInteractionsOperation(),
		sessionAttentionSummaryOperation(),
	}
}

func sessionPromptCancelOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/cancel",
		OperationID: "cancelSessionPrompt",
		Summary:     "Cancel the active session prompt",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Cancellation result", Body: contract.SessionPromptCancelResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func sessionWaitOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/wait",
		OperationID: "waitForSession",
		Summary:     "Wait for a session state",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.SessionWaitRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Wait completed or timed out", Body: contract.SessionWaitResponse{}},
			{Status: 400, Description: "Malformed wait request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Session not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Wait limit reached or registration in use", Body: contract.ErrorPayload{}},
			{Status: 410, Description: "Session gone or wait registration expired", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid wait request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Session wait is unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
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
			queryParam("workspace_id", "Workspace id or path", false),
			boolQueryParam("all_workspaces", "Use the explicit all-workspaces aggregate"),
			boolQueryParam("include_health", "Include metadata-only health for returned sessions"),
			enumQueryParam(
				"state",
				"Filter by exact session state",
				[]string{"starting", specActiveKey, "stopping", "stopped"},
			),
			enumQueryParam("type", "Filter by exact session type", sessionCatalogTypeValues()),
			queryParam("agent", "Filter by exact agent definition name", false),
			queryParam("parent", "Filter by exact parent session id", false),
			queryParam("root", "Filter by exact root session id (includes the root itself)", false),
			queryParam("worktree", "Filter by exact bound worktree id", false),
			queryParam("q", "Search session id, name, agent, provider, or channel", false),
			boolQueryParam("resumable", "Only list sessions eligible for explicit attach"),
			boolQueryParam("attention", "Only list sessions in the needs-you attention class"),
			queryParam("badge", "Filter by comma-separated exact session badges", false),
			enumQueryParam(
				"archive",
				"Archived session visibility",
				[]string{"exclude", "only", "include"},
			),
			enumQueryParam("sort", "Stable session ordering", []string{"recent", "last_activity", "attention"}),
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

func sessionPresenceOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/presence",
		OperationID: "updateSessionPresence",
		Summary:     "Update operator session presence",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.SessionPresenceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Presence lease acquired", Body: contract.SessionPresenceResponse{}},
			{Status: 204, Description: "Presence lease renewed or released"},
			{Status: 400, Description: "Invalid presence request", Body: contract.ErrorPayload{}},
			{Status: 403, Description: agentIdentityDeniedDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Session or lease not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Presence lease ownership mismatch", Body: contract.ErrorPayload{}},
			{Status: 503, Description: sessionAttentionUnavailableDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func sessionInteractionsOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/interactions",
		OperationID: "listSessionInteractions",
		Summary:     "List pending session interactions",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			enumQueryParam(
				"status",
				"Filter by interaction status",
				[]string{"pending", "orphaned", "resolved", "timed_out", "canceled"},
			),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionInteractionsResponse{}},
			{Status: 400, Description: "Invalid interaction status", Body: contract.ErrorPayload{}},
			{Status: 403, Description: agentIdentityDeniedDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Session not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: sessionAttentionUnavailableDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func sessionAttentionSummaryOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/sessions/attention-summary",
		OperationID: "getSessionAttentionSummary",
		Summary:     "Get exact session attention counts",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionAttentionSummaryPayload{}},
			{Status: 403, Description: agentIdentityDeniedDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: sessionAttentionUnavailableDescription, Body: contract.ErrorPayload{}},
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
		Parameters: []ParameterSpec{
			queryParam("workspace_id", "Workspace id or path", false),
			boolQueryParam("all_workspaces", "Subscribe to the explicit all-workspaces aggregate"),
			optionalHeaderParam("Last-Event-ID", "Resume after this catalog sequence"),
		},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Workspace-identified session catalog event stream",
				Bodies: responseBodiesOf(
					responseBodyOf[contract.SessionCatalogEventPayload](),
					responseBodyOf[contract.SessionAttentionEventPayload](),
					responseBodyOf[contract.OperatorNotificationEventPayload](),
				),
				ContentType: specContentTypeEventStream,
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Session catalog stream is unavailable", Body: contract.ErrorPayload{}},
		},
	}
}
