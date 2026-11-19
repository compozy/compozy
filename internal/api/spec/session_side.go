package spec

import "github.com/compozy/compozy/internal/api/contract"

func openSessionSideOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: "/api/workspaces/{workspace_id}/sessions/{session_id}/sides",
		AppAccess: AppAccessAllowed, OperationID: "openSessionSide", Summary: "Open a side chat without changing its source session",
		Tags: []string{specSessionsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Source session id")),
		RequestBody: contract.OpenSessionSideRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.SessionDeriveResponse{}},
			{Status: 200, Description: "Recorded side chat replayed", Body: contract.SessionDeriveResponse{}},
			{Status: 400, Description: "Invalid side chat question or identity", Body: contract.SessionDeriveErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.SessionDeriveErrorPayload{}},
			{Status: 409, Description: "Side chat refused", Body: contract.SessionDeriveErrorPayload{}},
			{Status: 422, Description: specDeriveCommittedFailureDescription, Body: contract.SessionDeriveErrorPayload{}},
			{Status: 503, Description: specNewWorkAdmissionUnavailableDescription, Body: contract.SessionDeriveErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.SessionDeriveErrorPayload{}},
		},
	}
}

func listSessionSidesOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: "/api/workspaces/{workspace_id}/sessions/{session_id}/sides",
		AppAccess: AppAccessAllowed, OperationID: "listSessionSides", Summary: "List all side chats attached to a session, newest updated first",
		Tags: []string{specSessionsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Source session id"),
			queryParam("archived", "Include closed side chats", false),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "Complete side chat list", Body: contract.SessionSidesResponse{}},
			{Status: 400, Description: "Invalid archive filter", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Side chats cannot list nested side chats", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
