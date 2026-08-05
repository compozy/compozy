package spec

import "github.com/compozy/compozy/internal/api/contract"

func archiveSessionOperationSpec() OperationSpec {
	spec := sessionArchiveOperationSpec("archive", "archiveSession", "Archive a stopped session")
	spec.Responses = append(spec.Responses, ResponseSpec{
		Status:      409,
		Description: "Session must be stopped before archiving",
		Body:        contract.ErrorPayload{},
	})
	return spec
}

func unarchiveSessionOperationSpec() OperationSpec {
	return sessionArchiveOperationSpec("unarchive", "unarchiveSession", "Restore an archived session")
}

func sessionArchiveOperationSpec(action string, operationID string, summary string) OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/" + action,
		OperationID: operationID,
		Summary:     summary,
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
