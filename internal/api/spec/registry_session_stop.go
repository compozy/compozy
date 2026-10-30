package spec

import "github.com/compozy/compozy/internal/api/contract"

func stopSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/stop",
		AppAccess:   AppAccessAllowed,
		OperationID: "stopSession",
		Summary:     "Stop a session without deleting persisted history",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		),
		RequestBody:         contract.StopSessionRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "Settled stop result or already stopped", Body: contract.SessionStopPayload{}},
			{Status: 202, Description: "Stop accepted", Body: contract.SessionStopPayload{}},
			{Status: 400, Description: "Invalid stop request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 409, Description: "Process cleanup could not be verified", Body: contract.ErrorPayload{}},
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
