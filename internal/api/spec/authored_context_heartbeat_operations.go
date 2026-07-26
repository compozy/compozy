package spec

import "github.com/compozy/agh/internal/api/contract"

func authoredContextHeartbeatOperations() []OperationSpec {
	return []OperationSpec{
		getAgentHeartbeatOperationSpec(),
		validateAgentHeartbeatOperationSpec(),
		putAgentHeartbeatOperationSpec(),
		deleteAgentHeartbeatOperationSpec(),
		listAgentHeartbeatHistoryOperationSpec(),
		rollbackAgentHeartbeatOperationSpec(),
		getAgentHeartbeatStatusOperationSpec(),
		wakeAgentHeartbeatOperationSpec(),
	}
}
func getAgentHeartbeatOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        authoredContextAPIAgentsAgentNameHeartbeatPath,
		OperationID: "getAgentHeartbeat",
		Summary:     "Inspect the resolved Heartbeat policy for an agent definition",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
			queryParam("workspace_id", "Workspace id", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatPolicyPayload{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      404,
				Description: authoredContextAgentOrWorkspaceNotFoundDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 422, Description: "Heartbeat policy is invalid", Body: contract.HeartbeatPolicyPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func validateAgentHeartbeatOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agents/{name}/heartbeat/validate",
		OperationID: "validateAgentHeartbeat",
		Summary:     "Validate a proposed HEARTBEAT.md body",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.HeartbeatValidateByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatPolicyPayload{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      404,
				Description: authoredContextAgentOrWorkspaceNotFoundDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      422,
				Description: authoredContextHeartbeatValidationFailedDescription,
				Body:        contract.HeartbeatPolicyPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func putAgentHeartbeatOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        authoredContextAPIAgentsAgentNameHeartbeatPath,
		OperationID: "putAgentHeartbeat",
		Summary:     "Create or replace HEARTBEAT.md through managed authoring",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.HeartbeatPutByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatMutationResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      404,
				Description: authoredContextAgentOrWorkspaceNotFoundDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      409,
				Description: authoredContextHeartbeatAuthoringConflictDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      422,
				Description: authoredContextHeartbeatValidationFailedDescription,
				Body:        contract.HeartbeatPolicyPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteAgentHeartbeatOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        authoredContextAPIAgentsAgentNameHeartbeatPath,
		OperationID: "deleteAgentHeartbeat",
		Summary:     "Delete HEARTBEAT.md through managed authoring",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.HeartbeatDeleteByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatMutationResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Agent, workspace, or policy not found", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: authoredContextHeartbeatAuthoringConflictDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 422, Description: "Invalid Heartbeat delete request", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listAgentHeartbeatHistoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/agents/{name}/heartbeat/history",
		OperationID: "listAgentHeartbeatHistory",
		Summary:     "List managed HEARTBEAT.md authoring revisions",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
			queryParam("workspace_id", "Workspace id", false),
			intQueryParam("limit", "Maximum number of revisions to return"),
			queryParam("cursor", "Revision cursor", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatHistoryResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      404,
				Description: authoredContextAgentOrWorkspaceNotFoundDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 422, Description: "Invalid Heartbeat history request", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func rollbackAgentHeartbeatOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agents/{name}/heartbeat/rollback",
		OperationID: "rollbackAgentHeartbeat",
		Summary:     "Rollback HEARTBEAT.md through managed authoring",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.HeartbeatRollbackByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatMutationResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      404,
				Description: "Agent, workspace, revision, or snapshot not found",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      409,
				Description: authoredContextHeartbeatAuthoringConflictDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      422,
				Description: authoredContextHeartbeatValidationFailedDescription,
				Body:        contract.HeartbeatPolicyPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getAgentHeartbeatStatusOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        authoredContextAPIAgentsAgentNameHeartbeatStatusPath,
		OperationID: "getAgentHeartbeatStatus",
		Summary:     "Read Heartbeat policy status, wake state, and optional session health",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
			queryParam("workspace_id", "Workspace id", false),
			queryParam("session_id", "Session id for wake state and health", false),
			boolQueryParam("include_session_health", "Include session health when a session id is supplied"),
			boolQueryParam("include_recent_wake_events", "Include recent wake audit rows"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatStatusResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Agent, workspace, or session not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Heartbeat status request is invalid", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func wakeAgentHeartbeatOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agents/{name}/heartbeat/wake",
		OperationID: "wakeAgentHeartbeat",
		Summary:     "Request one advisory Heartbeat wake for an eligible session",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.HeartbeatWakeByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HeartbeatWakeResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Agent, workspace, policy, or session not found", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Wake skipped or coalesced by policy and health gates",
				Body:        contract.HeartbeatWakeResponse{},
			},
			{Status: 422, Description: "Invalid Heartbeat wake request", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
