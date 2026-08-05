package spec

import "github.com/compozy/compozy/internal/api/contract"

func listSessionEventsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/events",
		OperationID: "listSessionEvents",
		Summary:     "List persisted session events",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			dateTimeQueryParam("since", "Only events emitted since this timestamp"),
			intQueryParam(
				"limit",
				"Maximum number of records to return; defaults to the newest 200 and is capped at 1000",
			),
			afterSequenceQueryParam("Only return events after this sequence number"),
			queryParam("type", "Event type", false),
			queryParam("agent_name", "Agent name", false),
			queryParam("turn_id", "Turn id", false),
			enumQueryParam(
				"archive",
				"Select active, archived, or all events",
				[]string{specActiveKey, specArchivedKey, specAllKey},
			),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionEventsResponse{}},
			{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func getSessionHistoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/history",
		OperationID: "getSessionHistory",
		Summary:     "List grouped session turn history",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			dateTimeQueryParam("since", "Only events emitted since this timestamp"),
			intQueryParam(
				"limit",
				"Maximum number of turns to return; defaults to the newest 200 and is capped at 1000",
			),
			afterSequenceQueryParam("Only return events after this sequence number"),
			queryParam("type", "Event type", false),
			queryParam("agent_name", "Agent name", false),
			queryParam("turn_id", "Turn id", false),
			enumQueryParam(
				"archive",
				"Select active, archived, or all events",
				[]string{specActiveKey, specArchivedKey, specAllKey},
			),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionHistoryResponse{}},
			{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
