package spec

import "github.com/compozy/compozy/internal/api/contract"

func registrySessionOperations() []OperationSpec {
	return []OperationSpec{
		createSessionOperationSpec(),
		getSessionByIDOperationSpec(),
		getSessionOwnerOperationSpec(),
		getSessionOperationSpec(),
		renameSessionOperationSpec(),
		getSessionCommandsOperationSpec(),
		deleteSessionOperationSpec(),
		stopSessionOperationSpec(),
		archiveSessionOperationSpec(),
		unarchiveSessionOperationSpec(),
		attachSessionOperationSpec(),
		uploadSessionAttachmentOperationSpec(),
		readSessionAttachmentBytesOperationSpec(),
		deleteSessionAttachmentOperationSpec(),
		forkSessionWorktreeOperationSpec(),
		setSessionRuntimeOperationSpec(),
		clearSessionRuntimeOperationSpec(),
		sendSessionPromptOperationSpec(),
		steerSessionPromptOperationSpec(),
		listSessionInputsOperationSpec(),
		replaceSessionInputOperationSpec(),
		promoteSessionInputOperationSpec(),
		cancelQueuedSessionPromptOperationSpec(),
		getSessionRecapOperationSpec(),
		getSessionUsageOperationSpec(),
		repairSessionOperationSpec(),
		listSessionEventsOperationSpec(),
		getSessionHistoryOperationSpec(),
		approveSessionOperationSpec(),
	}
}
func renameSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specWorkspaceSessionPath,
		OperationID: "renameSession",
		Summary:     "Rename one user session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.RenameSessionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 400, Description: "Invalid session name", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Session rename unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func forkSessionWorktreeOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/worktree-fork",
		OperationID: "forkSessionToWorktree",
		Summary:     "Fork a live session into a worktree",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Origin session id"),
		},
		RequestBody: contract.ForkSessionWorktreeRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.SessionResponse{}},
			{Status: 400, Description: "Confirmation or one worktree target is missing", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Session, workspace, or worktree not found", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Origin session is busy or worktree is unavailable",
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNewWorkAdmissionUnavailableDescription, Body: contract.ErrorPayload{}},
		},
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
func getSessionOwnerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/sessions/{session_id}/owner",
		OperationID: "getSessionOwner",
		Summary:     "Get a session workspace owner projection",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionOwner{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
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
		Path:        specWorkspaceSessionPath,
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
		Path:        specWorkspaceSessionPath,
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
			promptSuccessResponse(202, "Prompt queued, steering accepted, or Goal command started"),
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
			{Status: 202, Description: "Prompt steering accepted", Body: contract.SendPromptResultResponse{}},
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

func listSessionInputsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue",
		OperationID: "listSessionInputs",
		Summary:     "List pending session input",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Pending session input", Body: contract.SessionInputListResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func replaceSessionInputOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}",
		OperationID: "replaceSessionInput",
		Summary:     "Replace queued session input atomically",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			pathParam("queue_entry_id", "Queue entry id"),
		},
		RequestBody: contract.ReplaceSessionInputRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Queued input replaced", Body: contract.SessionInputResponse{}},
			{Status: 400, Description: "Invalid input replacement", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func promoteSessionInputOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}/steer",
		OperationID: "promoteSessionInput",
		Summary:     "Promote queued session input to steering",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			pathParam("queue_entry_id", "Queue entry id"),
		},
		RequestBody: contract.PromoteSessionInputRequest{},
		Responses: []ResponseSpec{
			{Status: 202, Description: "Queued input promoted to steering", Body: contract.SendPromptResultResponse{}},
			{Status: 400, Description: "Invalid steering promotion", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specSessionPromptConflictDescription, Body: contract.ErrorPayload{}},
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
