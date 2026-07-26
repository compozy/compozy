package spec

import (
	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func supplementalOperationSpecs() []OperationSpec {
	specs := sessionCatalogOperations()
	specs = append(specs, toolApprovalGrantOperationSpecs()...)
	specs = append(specs, automationSuggestionOperationSpecs()...)
	return append(specs, clarifyOperationSpecs()...)
}

func clarifyOperationSpecs() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/clarifications",
			OperationID: "listSessionClarifications",
			Summary:     "List the live pending clarification for a session",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ClarificationsResponse{}},
				{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Clarification broker is not configured", Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/clarifications/{request_id}/answer",
			OperationID: "answerSessionClarification",
			Summary:     "Answer a live session clarification",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
				pathParam("request_id", "Clarification request id"),
			},
			RequestBody: contract.ClarificationAnswerRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: toolspkg.ClarifyAnswer{}},
				{Status: 400, Description: "Invalid clarification answer", Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Session or clarification not found", Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Clarification is no longer answerable", Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Clarification broker is not configured", Body: contract.ErrorPayload{}},
			},
		},
	}
}
