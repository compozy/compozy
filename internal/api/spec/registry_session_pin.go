package spec

import "github.com/compozy/compozy/internal/api/contract"

// pinSessionOperationSpec describes pinning (POST) or unpinning (DELETE) one session.
func pinSessionOperationSpec(pin bool) OperationSpec {
	method, operationID := httpMethodPost, "pinSession"
	summary := "Pin a session above the session list; pinning a pinned session changes nothing"
	if !pin {
		method, operationID = httpMethodDelete, "unpinSession"
		summary = "Unpin a session so it returns to its place in the list; an unpinned session is unchanged"
	}
	return OperationSpec{
		Method:      method,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/pin",
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
			{
				Status:      400,
				Description: "Conflicting profile selectors (profile_selection_conflict)",
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status: 409,
				Description: "Subagent sessions follow their parent and an always-on agent's main session " +
					"lives in its avatar, so neither can be pinned",
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
