package spec

import "github.com/compozy/compozy/internal/api/contract"

func compactSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: "/api/workspaces/{workspace_id}/sessions/{session_id}/compact",
		OperationID: "compactSession", Summary: "Request advertised native agent compaction (experimental)", Stability: "experimental",
		Tags: []string{specSessionsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Session id")},
		RequestBody: contract.SessionCompactRequest{},
		Responses: []ResponseSpec{
			{Status: 202, Description: "Compaction accepted", Body: contract.SessionCompactResponse{}},
			{Status: 409, Description: "session_busy or compaction_unsupported", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
