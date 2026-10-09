package spec

import "github.com/compozy/compozy/internal/api/contract"

func registrySubagentOperations() []OperationSpec {
	const subagentPath = "/api/workspaces/{workspace_id}/subagents/{subagent_id}"
	return []OperationSpec{
		workspaceSessionCatalogListOperation(),
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/subagents",
			OperationID: "listSessionSubagents", Summary: "List a session's subagents", Tags: []string{specSessionsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Parent session id"),
				enumQueryParam("origin", "Subagent origin", []string{"delegated", "provider_native"}),
				queryParam("status", "Comma-separated subagent statuses", false),
				intQueryParam("limit", "Subagents per page (default 50, maximum 200)"),
				queryParam("cursor", "Opaque next_cursor from the previous page", false),
			},
			Responses: subagentResponses(200, contract.SubagentListPayload{}),
		},
		{
			Method: httpMethodGet, Path: subagentPath, OperationID: "getSubagent",
			Summary: "Read one subagent", Tags: []string{specSessionsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{pathParam("workspace_id", "Workspace id"), pathParam("subagent_id", "Subagent id")},
			Responses:  subagentResponses(200, contract.SubagentPayload{}),
		},
		{
			Method: httpMethodPost, Path: subagentPath + "/cancel", OperationID: "cancelSubagent",
			Summary: "Cancel a delegated subagent", Tags: []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("workspace_id", "Workspace id"), pathParam("subagent_id", "Subagent id")},
			RequestBody: contract.SubagentCancelRequest{},
			Responses: append(subagentResponses(202, contract.SubagentCancelPayload{}),
				ResponseSpec{Status: 409, Description: "Subagent is not cancelable", Body: contract.ErrorPayload{}}),
		},
	}
}

func subagentResponses(status int, body any) []ResponseSpec {
	return []ResponseSpec{
		{Status: status, Description: "Success", Body: body},
		{Status: 400, Description: "Invalid request", Body: contract.ErrorPayload{}},
		{Status: 403, Description: "Capability denied", Body: contract.ErrorPayload{}},
		{Status: 404, Description: "Session or subagent not found", Body: contract.ErrorPayload{}},
		{Status: 503, Description: "Subagents unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}

func workspaceSessionCatalogListOperation() OperationSpec {
	operation := sessionCatalogListOperation()
	operation.Path = "/api/workspaces/{workspace_id}/sessions"
	operation.OperationID = "listWorkspaceSessions"
	parameters := []ParameterSpec{pathParam("workspace_id", "Workspace id")}
	for _, parameter := range operation.Parameters {
		if parameter.Name != "workspace_id" && parameter.Name != "all_workspaces" {
			parameters = append(parameters, parameter)
		}
	}
	operation.Parameters = parameters
	return operation
}
