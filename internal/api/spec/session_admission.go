package spec

import "github.com/compozy/compozy/internal/api/contract"

const specNewWorkAdmissionUnavailableDescription = "New-work admission is unavailable while " +
	"the daemon is draining"

func sessionAdmissionOperations() []OperationSpec {
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
