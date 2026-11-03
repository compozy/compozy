package spec

import "github.com/compozy/compozy/internal/api/contract"

func moveSessionToLaterOperationSpec() OperationSpec {
	return sessionLaterOperationSpec(
		"later",
		"moveSessionToLater",
		"Stop a session and its children and park them in Later, with an optional reason and return time",
		contract.MoveSessionToLaterRequest{},
		"Invalid Move to Later request",
		"Subagents move with their parent, or the session tree could not be stopped",
	)
}

func resumeSessionFromLaterOperationSpec() OperationSpec {
	return sessionLaterOperationSpec(
		"resume-later",
		"resumeSessionFromLater",
		"Return a parked session, and optionally its parked children, to the normal flow",
		contract.ResumeSessionFromLaterRequest{},
		"Invalid Back from Later request",
		"The session could not leave Later",
	)
}

func markSessionSeenOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/mark-seen",
		AppAccess:   AppAccessAllowed,
		OperationID: "markSessionSeen",
		Summary: "Mark a session's Needs you as seen until its next occurrence; a session " +
			"waiting on an answer, a sign-in, or a stop is refused",
		Tags:       []string{specSessionsKey},
		Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{
				Status:      400,
				Description: "Conflicting profile selectors (profile_selection_conflict)",
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status: 409,
				Description: "The session still waits on an answer, a sign-in, or a stop, " +
					"or a newer occurrence replaced the one checked",
				Body: contract.ErrorPayload{},
			},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func sessionLaterOperationSpec(
	action string,
	operationID string,
	summary string,
	body any,
	invalidDescription string,
	conflictDescription string,
) OperationSpec {
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
		RequestBody:         body,
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 400, Description: invalidDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: conflictDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
