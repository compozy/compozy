package spec

import "github.com/compozy/compozy/internal/api/contract"

func clearSessionInputsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue",
		OperationID: "clearSessionInputs", Summary: "Clear queued input with attributed per-entry traces",
		Tags: []string{specSessionsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Session id")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Per-entry clear outcomes", Body: contract.SessionInputClearResponse{}},
			{Status: 403, Description: "Forbidden", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
