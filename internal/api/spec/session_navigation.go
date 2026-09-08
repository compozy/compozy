package spec

import "github.com/compozy/compozy/internal/api/contract"

func sessionNavigationOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/search",
			OperationID: "searchSessionTranscript",
			Summary:     "Search retained transcript messages with bounded literal matches",
			Tags:        []string{specSessionsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
				{
					Name:        "q",
					In:          "query",
					Description: "Literal case-insensitive text; 1 to 4096 bytes",
					Required:    true,
					Kind:        "string",
				},
				intQueryParam(
					"limit",
					"Maximum matches; defaults to 200, capped at 1000; truncated reports omitted matches",
				),
			},
			Responses: sessionNavigationResponses(contract.SessionTranscriptSearchResponse{}),
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/outline",
			OperationID: "getSessionTranscriptOutline",
			Summary:     "Get the full retained operator-message trail with reply previews",
			Tags:        []string{specSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			},
			Responses: sessionNavigationResponses(contract.SessionTranscriptOutlineResponse{}),
		},
	}
}

func sessionNavigationResponses(body any) []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "OK", Body: body},
		{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
		{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		{Status: 503, Description: specTranscriptIncompatibleDescription, Body: contract.ErrorPayload{}},
	}
}
