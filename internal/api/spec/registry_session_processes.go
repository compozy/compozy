package spec

import "github.com/compozy/compozy/internal/api/contract"

func sessionProcessPolicyParameter() ParameterSpec {
	return ParameterSpec{Name: "processes", In: "query", Kind: specStringKey,
		Enum: []string{"keep", "terminate"}, Description: "Keep (default) or terminate attributed session processes"}
}

func sessionProcessOperations() []OperationSpec {
	const path = "/api/workspaces/{workspace_id}/sessions/{session_id}/processes"
	return []OperationSpec{
		{
			Method: httpMethodPost, Path: "/api/processes/terminate", AppAccess: AppAccessDenied,
			OperationID: "terminateProcesses", Summary: "Terminate identity-verified processes with missing or retained session owners",
			Tags: []string{specSessionsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(), RequestBody: contract.TerminateProcessesRequest{},
			Responses: sessionProcessResponses(contract.TerminateSessionProcessesResponse{}),
		},
		{
			Method:      httpMethodGet,
			Path:        path,
			AppAccess:   AppAccessAllowed,
			OperationID: "listSessionProcesses",
			Summary:     "List attributed session processes",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			),
			Responses: sessionProcessResponses(contract.SessionProcessesResponse{}),
		},
		{
			Method:      httpMethodPost,
			Path:        path + "/terminate",
			AppAccess:   AppAccessAllowed,
			OperationID: "terminateSessionProcesses",
			Summary:     "Terminate identity-verified session processes",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			),
			RequestBody: contract.TerminateSessionProcessesRequest{},
			Responses:   sessionProcessResponses(contract.TerminateSessionProcessesResponse{}),
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/processes",
			AppAccess:   AppAccessDenied,
			OperationID: "listProcesses",
			Summary:     "List processes grouped by session, including left-running processes",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(
				boolQueryParam("all_workspaces", "Include all projects and User sessions"),
				boolQueryParam("orphaned", "Only processes of stopped, archived, deleted or unknown sessions"),
			),
			Responses: sessionProcessResponses(contract.ProcessesResponse{}),
		},
	}
}

func sessionProcessResponses(body any) []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "Success", Body: body},
		{Status: 400, Description: "Invalid process request", Body: contract.ErrorPayload{}},
		{Status: 403, Description: "Process scope denied", Body: contract.ErrorPayload{}},
		{Status: 404, Description: "Session not found", Body: contract.ErrorPayload{}},
		{Status: 503, Description: "Process scanner unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}
