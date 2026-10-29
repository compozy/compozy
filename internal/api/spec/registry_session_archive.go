package spec

import "github.com/compozy/compozy/internal/api/contract"

func archiveSessionOperationSpec() OperationSpec {
	spec := sessionArchiveOperationSpec(
		"archive",
		"archiveSession",
		"Archive a stopped session, or stop and archive a live session with stop=true",
	)
	// A query opt-in keeps the generated SDK archive signatures unchanged.
	spec.Parameters = withProfileSelector(
		sessionProcessPolicyParameter(),
		pathParam("workspace_id", "Workspace id"),
		pathParam("session_id", "Session id"),
		boolQueryParam(
			"stop",
			"Stop a live session through the normal stop path before archiving it; a failed stop leaves it unarchived",
		),
	)
	spec.Responses = append(spec.Responses,
		ResponseSpec{Status: 400, Description: "Invalid archive request", Body: contract.ErrorPayload{}},
		ResponseSpec{
			Status:      409,
			Description: "Session must be stopped before archiving, or the requested stop failed",
			Body:        contract.ErrorPayload{},
		},
	)
	return spec
}

func unarchiveSessionOperationSpec() OperationSpec {
	return sessionArchiveOperationSpec("unarchive", "unarchiveSession", "Restore an archived session")
}

// sessionArchiveOperationSpec shares the explicit owner selector across archive and restore.
func sessionArchiveOperationSpec(action string, operationID string, summary string) OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/" + action,
		AppAccess:   AppAccessAllowed,
		OperationID: operationID,
		Summary:     summary,
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		),
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
