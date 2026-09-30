package spec

import "github.com/compozy/compozy/internal/api/contract"

const specNewWorkAdmissionUnavailableDescription = "New-work admission is unavailable while " +
	"the daemon is draining"

func sessionAdmissionOperations() []OperationSpec {
	return append(sessionConversationAdmissionOperations(), sessionDeriveOperations()...)
}

func sessionConversationAdmissionOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodPost,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/clear",
			OperationID: "clearSessionConversation",
			Summary:     "Clear conversation history and restart the session context",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
				{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Session conversation cannot be cleared", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specNewWorkAdmissionUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/rewind",
			OperationID: "rewindSessionConversation",
			Summary:     "Rewind a conversation before one durable user message",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			},
			RequestBody: contract.SessionConversationRewindRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.SessionConversationRewindResponse{}},
				{Status: 400, Description: "Invalid rewind request", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Session conversation cannot be rewound", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specNewWorkAdmissionUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}

func sessionDeriveOperations() []OperationSpec {
	return []OperationSpec{
		continueSessionOperationSpec(),
		forkSessionOperationSpec(),
		previewSessionDeriveOperationSpec(),
	}
}

func continueSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/continue",
		OperationID: "continueSession",
		Summary:     "Continue a session with another agent, runtime, or declared route",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Source session id"),
		},
		RequestBody: contract.ContinueSessionRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.SessionDeriveResponse{}},
			{Status: 200, Description: "Recorded outcome replayed", Body: contract.SessionDeriveResponse{}},
			{Status: 400, Description: "Invalid continue request", Body: contract.SessionDeriveErrorPayload{}},
			{Status: 404, Description: "Session or agent not found", Body: contract.SessionDeriveErrorPayload{}},
			{Status: 409, Description: "Session cannot be continued", Body: contract.SessionDeriveErrorPayload{}},
			{
				Status:      422,
				Description: specDeriveCommittedFailureDescription,
				Body:        contract.SessionDeriveErrorPayload{},
			},
			{
				Status:      503,
				Description: specNewWorkAdmissionUnavailableDescription,
				Body:        contract.SessionDeriveErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.SessionDeriveErrorPayload{},
			},
		},
	}
}

func forkSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/fork",
		OperationID: "forkSession",
		Summary:     "Fork a session with the same agent, whole or through one user message and its turn",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Source session id"),
		},
		RequestBody: contract.ForkSessionRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.SessionDeriveResponse{}},
			{Status: 200, Description: "Recorded outcome replayed", Body: contract.SessionDeriveResponse{}},
			{Status: 400, Description: "Invalid fork request", Body: contract.SessionDeriveErrorPayload{}},
			{Status: 404, Description: "Session or message not found", Body: contract.SessionDeriveErrorPayload{}},
			{Status: 409, Description: "Session cannot be forked", Body: contract.SessionDeriveErrorPayload{}},
			{
				Status:      422,
				Description: specDeriveCommittedFailureDescription,
				Body:        contract.SessionDeriveErrorPayload{},
			},
			{
				Status:      503,
				Description: specNewWorkAdmissionUnavailableDescription,
				Body:        contract.SessionDeriveErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.SessionDeriveErrorPayload{},
			},
		},
	}
}

func previewSessionDeriveOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/derive/preview",
		OperationID: "previewSessionDerive",
		Summary:     "Preview what a continue or fork of a session would carry",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Source session id"),
			queryParam("message_id", "Durable user message to cut through (fork from here)", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionDerivePreviewResponse{}},
			{Status: 400, Description: "Session cannot be continued or forked", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Session or message not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Session is archived", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

// specDeriveCommittedFailureDescription documents the runtime/auth failure that can
// follow the child's commit (first-message admission); child_session_id names it.
const specDeriveCommittedFailureDescription = "Runtime or authentication failure; when the new session " +
	"was already created the error carries child_session_id and a retry with the same idempotency_key returns it"
