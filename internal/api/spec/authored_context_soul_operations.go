package spec

import "github.com/compozy/agh/internal/api/contract"

func authoredContextSoulOperations() []OperationSpec {
	return []OperationSpec{
		getAgentSoulOperationSpec(),
		validateAgentSoulOperationSpec(),
		getAgentDefinitionSoulOperationSpec(),
		validateAgentDefinitionSoulOperationSpec(),
		putAgentSoulOperationSpec(),
		deleteAgentSoulOperationSpec(),
		listAgentSoulHistoryOperationSpec(),
		rollbackAgentSoulOperationSpec(),
		refreshSessionSoulOperationSpec(),
	}
}
func getAgentSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        authoredContextAPIAgentSoulPath,
		OperationID: "getAgentSoul",
		Summary:     "Inspect the resolved Soul read model for the calling agent",
		Tags:        []string{authoredContextAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulPayload{}},
			{
				Status:      401,
				Description: authoredContextAgentCallerIdentityIsMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Caller session or agent not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Soul is invalid", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func validateAgentSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/soul/validate",
		OperationID: "validateAgentSoul",
		Summary:     "Validate a proposed Soul body or the calling agent's current Soul",
		Tags:        []string{authoredContextAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.AgentSoulValidateRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulPayload{}},
			{
				Status:      401,
				Description: authoredContextAgentCallerIdentityIsMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Caller session or agent not found", Body: contract.ErrorPayload{}},
			{
				Status:      422,
				Description: authoredContextSoulValidationFailedDescription,
				Body:        contract.AgentSoulPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getAgentDefinitionSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        authoredContextAPIAgentsAgentNameSoulPath,
		OperationID: "getAgentDefinitionSoul",
		Summary:     "Inspect the resolved Soul read model for an agent definition",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
			queryParam("workspace_id", "Workspace id", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulPayload{}},
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
			{Status: 422, Description: "Soul is invalid", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func validateAgentDefinitionSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agents/{name}/soul/validate",
		OperationID: "validateAgentDefinitionSoul",
		Summary:     "Validate a proposed Soul body for an agent definition",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.AgentSoulValidateByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulPayload{}},
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
				Description: authoredContextSoulValidationFailedDescription,
				Body:        contract.AgentSoulPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func putAgentSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        authoredContextAPIAgentsAgentNameSoulPath,
		OperationID: "putAgentSoul",
		Summary:     "Create or replace SOUL.md through managed authoring",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.AgentSoulPutByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulMutationResponse{}},
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
			{Status: 409, Description: authoredContextSoulAuthoringConflictDescription, Body: contract.ErrorPayload{}},
			{
				Status:      422,
				Description: authoredContextSoulValidationFailedDescription,
				Body:        contract.AgentSoulPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteAgentSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        authoredContextAPIAgentsAgentNameSoulPath,
		OperationID: "deleteAgentSoul",
		Summary:     "Delete SOUL.md through managed authoring",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.AgentSoulDeleteByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulMutationResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Agent, workspace, or Soul file not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: authoredContextSoulAuthoringConflictDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid Soul delete request", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listAgentSoulHistoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/agents/{name}/soul/history",
		OperationID: "listAgentSoulHistory",
		Summary:     "List managed SOUL.md authoring revisions",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
			queryParam("workspace_id", "Workspace id", false),
			intQueryParam("limit", "Maximum number of revisions to return"),
			queryParam("cursor", "Revision cursor", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulHistoryResponse{}},
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
			{Status: 422, Description: "Invalid Soul history request", Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func rollbackAgentSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agents/{name}/soul/rollback",
		OperationID: "rollbackAgentSoul",
		Summary:     "Rollback SOUL.md through managed authoring",
		Tags:        []string{authoredContextAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.AgentSoulRollbackByPathRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulMutationResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Agent, workspace, or revision not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: authoredContextSoulAuthoringConflictDescription, Body: contract.ErrorPayload{}},
			{
				Status:      422,
				Description: authoredContextSoulValidationFailedDescription,
				Body:        contract.AgentSoulPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func refreshSessionSoulOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/soul/refresh",
		OperationID: "refreshSessionSoul",
		Summary:     "Refresh an idle session's Soul snapshot through body-level CAS",
		Tags:        []string{authoredContextSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.SessionSoulRefreshRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentSoulPayload{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: authoredContextSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Session is not idle or Soul digest is stale", Body: contract.ErrorPayload{}},
			{
				Status:      422,
				Description: authoredContextSoulValidationFailedDescription,
				Body:        contract.AgentSoulPayload{},
			},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
