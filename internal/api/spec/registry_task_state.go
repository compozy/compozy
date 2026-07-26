package spec

import "github.com/compozy/agh/internal/api/contract"

func registryTaskStateOperations() []OperationSpec {
	return []OperationSpec{
		startTaskRunOperationSpec(),
		attachTaskRunSessionOperationSpec(),
		completeTaskRunOperationSpec(),
		failTaskRunOperationSpec(),
		cancelTaskRunOperationSpec(),
		getTaskTimelineOperationSpec(),
		streamTaskOperationSpec(),
		getTaskTreeOperationSpec(),
		approveTaskOperationSpec(),
		rejectTaskOperationSpec(),
		markTaskReadOperationSpec(),
		archiveTaskOperationSpec(),
		dismissTaskOperationSpec(),
		getTaskDashboardOperationSpec(),
		getTaskInboxOperationSpec(),
		getObserveOverviewOperationSpec(),
	}
}
func startTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/task-runs/{id}/start",
		OperationID: "startTaskRun",
		Summary:     "Start one claimed task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.StartTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunResponse{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run start conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run start request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func attachTaskRunSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/task-runs/{id}/attach-session",
		OperationID: "attachTaskRunSession",
		Summary:     "Attach an existing session to one task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.AttachTaskRunSessionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunResponse{}},
			{Status: 404, Description: "Task run or session not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Attach-session conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid attach-session request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func completeTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/task-runs/{id}/complete",
		OperationID: "completeTaskRun",
		Summary:     "Complete one running task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.CompleteTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunResponse{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run completion conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run completion request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func failTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/task-runs/{id}/fail",
		OperationID: "failTaskRun",
		Summary:     "Fail one task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.FailTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunResponse{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run failure conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run failure request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func cancelTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/task-runs/{id}/cancel",
		OperationID: "cancelTaskRun",
		Summary:     "Cancel one task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.CancelTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunResponse{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run cancel conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run cancel request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskTimelineOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/tasks/{id}/timeline",
		OperationID: "getTaskTimeline",
		Summary:     "Get one task timeline",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			afterSequenceQueryParam("Return only events after the supplied sequence"),
			intQueryParam("limit", "Maximum number of timeline items to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskTimelineResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid timeline query", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func streamTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/tasks/{id}/stream",
		OperationID: "streamTask",
		Summary:     "Stream task-native live events for one task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			afterSequenceQueryParam("Replay events after the supplied task stream sequence"),
		},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Standard SSE message stream; the payload type identifies each task event",
				Body:        contract.TaskStreamEventPayload{},
				ContentType: specContentTypeEventStream,
			},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task stream query", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskTreeOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/tasks/{id}/tree",
		OperationID: "getTaskTree",
		Summary:     "Get one task tree live view",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskTreeResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskIDDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func approveTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/approve",
		OperationID: "approveTask",
		Summary:     "Approve one approval-gated task and enqueue executable work",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody:         contract.TaskExecutionRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TaskExecutionResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task approval conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task approval request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func rejectTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/reject",
		OperationID: "rejectTask",
		Summary:     "Reject one approval-gated task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task rejection conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task rejection request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func markTaskReadOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/triage/read",
		OperationID: "markTaskRead",
		Summary:     "Mark one task inbox item as read",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskTriageStateResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskTriageRequestDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func archiveTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/triage/archive",
		OperationID: "archiveTask",
		Summary:     "Archive one task inbox item",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskTriageStateResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskTriageRequestDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func dismissTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/triage/dismiss",
		OperationID: "dismissTask",
		Summary:     "Dismiss one task inbox item",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskTriageStateResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskTriageRequestDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskDashboardOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/observe/tasks/dashboard",
		OperationID: "getTaskDashboard",
		Summary:     "Get the observer-backed task dashboard",
		Tags:        []string{specObserveKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Filter by task scope", taskScopeValues()),
			queryParam(specWorkspaceKey, "Filter by workspace path, name, or ID", false),
			enumQueryParam("owner_kind", "Filter by owner kind", taskOwnerKindValues()),
			queryParam("owner_ref", "Filter by owner reference", false),
			queryParam("participation_channel", "Filter by resolved participation channel", false),
			enumQueryParam("origin_kind", "Filter by task origin kind", taskOriginKindValues()),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskDashboardResponse{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task dashboard query", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specObserveNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskInboxOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/observe/tasks/inbox",
		OperationID: "getTaskInbox",
		Summary:     "Get the observer-backed task inbox",
		Tags:        []string{specObserveKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Filter by task scope", taskScopeValues()),
			queryParam(specWorkspaceKey, "Filter by workspace path, name, or ID", false),
			enumQueryParam("owner_kind", "Filter by owner kind", taskOwnerKindValues()),
			queryParam("owner_ref", "Filter by owner reference", false),
			enumQueryParam("lane", "Filter by inbox lane", taskInboxLaneValues()),
			enumQueryParam("status", "Filter by canonical task status", taskStatusValues()),
			enumQueryParam("priority", "Filter by task priority", taskPriorityValues()),
			boolQueryParam("unread", "Filter by unread state; false selects read items"),
			queryParam("query", "Filter by task title or identifier", false),
			queryParam("cursor", "Opaque actor- and query-bound continuation cursor", false),
			intQueryParam("limit", "Page size from 1 to 200 (default 50)"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskInboxResponse{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 400, Description: "Invalid task inbox query or cursor", Body: contract.ErrorPayload{}},
			{Status: 410, Description: workspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specObserveNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
