package spec

import "github.com/compozy/agh/internal/api/contract"

func registrySessionOperations() []OperationSpec {
	return []OperationSpec{
		createSessionOperationSpec(),
		getSessionByIDOperationSpec(),
		getSessionOperationSpec(),
		deleteSessionOperationSpec(),
		stopSessionOperationSpec(),
		attachSessionOperationSpec(),
		sendSessionPromptOperationSpec(),
		interruptSessionPromptOperationSpec(),
		steerSessionPromptOperationSpec(),
		cancelQueuedSessionPromptOperationSpec(),
		getSessionRecapOperationSpec(),
		getSessionUsageOperationSpec(),
		repairSessionOperationSpec(),
		listSessionEventsOperationSpec(),
		getSessionHistoryOperationSpec(),
		approveSessionOperationSpec(),
	}
}
func createSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specSessionsPath,
		OperationID: "createSession",
		Summary:     "Create a session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateSessionRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.SessionResponse{}},
			{Status: 400, Description: "Invalid create request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Session creation conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNewWorkAdmissionUnavailableDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSessionByIDOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/sessions/{session_id}",
		OperationID: "getSessionByID",
		Summary:     "Get one session snapshot by id",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("session_id", "Session id"),
			boolQueryParam("include_health", "Include metadata-only session health when available"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}",
		OperationID: "getSession",
		Summary:     "Get one session snapshot",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			boolQueryParam("include_health", "Include metadata-only session health when available"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}",
		OperationID: "deleteSession",
		Summary:     "Delete one session and remove it from persisted history",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func stopSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/stop",
		OperationID: "stopSession",
		Summary:     "Stop a session without deleting persisted history",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func attachSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/attach",
		OperationID: "attachSession",
		Summary:     "Attach to a resumable live session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody:         contract.AttachSessionRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionAttachResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Session cannot be attached", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func sendSessionPromptOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt",
		OperationID: "sendSessionPrompt",
		Summary:     "Send, queue, interrupt, or steer a session prompt",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.SendPromptRequest{},
		Responses: []ResponseSpec{
			promptSuccessResponse(200, "Prompt accepted or Goal command completed"),
			promptSuccessResponse(202, "Prompt queued, staged, or Goal command started"),
			{Status: 400, Description: "Invalid prompt request", Body: contract.ErrorPayload{}},
			promptGoalErrorResponse(404, specSessionNotFoundDescription),
			promptGoalErrorResponse(409, specSessionPromptConflictDescription),
			{Status: 413, Description: "Session input queue is full", Body: contract.ErrorPayload{}},
			promptGoalErrorResponse(422, "Goal command is invalid or cannot transition"),
			{Status: 503, Description: specNewWorkAdmissionUnavailableDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func interruptSessionPromptOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/interrupt",
		OperationID: "interruptSessionPrompt",
		Summary:     "Interrupt the active prompt turn for a session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Prompt interrupted", Body: contract.SendPromptResultResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specSessionPromptConflictDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func steerSessionPromptOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/steer",
		OperationID: "steerSessionPrompt",
		Summary:     "Stage steering input for the active session turn",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.SteerPromptRequest{},
		Responses: []ResponseSpec{
			{Status: 202, Description: "Prompt steering staged", Body: contract.SendPromptResultResponse{}},
			{Status: 400, Description: "Invalid steer request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specSessionPromptConflictDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func cancelQueuedSessionPromptOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}",
		OperationID: "cancelQueuedSessionPrompt",
		Summary:     "Cancel a queued session prompt entry",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			pathParam("queue_entry_id", "Queue entry id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Queued prompt canceled", Body: contract.SendPromptResultResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSessionRecapOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/recap",
		OperationID: "getSessionRecap",
		Summary:     "Get a deterministic session recap",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			intQueryParam("limit", "Maximum recent messages to include"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionRecapResponse{}},
			{Status: 400, Description: "Invalid recap query", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSessionUsageOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/usage",
		OperationID: "getSessionUsage",
		Summary:     "Get aggregated token usage for a session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionUsageResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func repairSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/repair",
		OperationID: "repairSession",
		Summary:     "Inspect and repair an interrupted session transcript",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			boolQueryParam("dry_run", "Report planned repairs without persisting new events"),
			boolQueryParam("force", "Allow repair for stopped sessions whose stop reason is not crash or error"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionRepairResponse{}},
			{Status: 400, Description: "Invalid repair options", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
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
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionHistoryResponse{}},
			{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func approveSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/approve",
		OperationID: "approveSession",
		Summary:     "Approve or deny an interactive permission request",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.ApproveSessionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionApprovalResponse{}},
			{Status: 400, Description: "Invalid approval request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
