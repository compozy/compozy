package spec

import "github.com/compozy/agh/internal/api/contract"

func registryTaskLifecycleOperations() []OperationSpec {
	return []OperationSpec{
		publishTaskOperationSpec(),
		startTaskOperationSpec(),
		cancelTaskOperationSpec(),
		pauseTaskOperationSpec(),
		resumeTaskOperationSpec(),
		createChildTaskOperationSpec(),
		addTaskDependencyOperationSpec(),
		removeTaskDependencyOperationSpec(),
		listTaskRunsOperationSpec(),
		enqueueTaskRunOperationSpec(),
		getTaskRunOperationSpec(),
		taskRunConversationStreamOperation(),
	}
}
func publishTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/publish",
		OperationID: "publishTask",
		Summary:     "Publish one draft task and enqueue executable work",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.TaskExecutionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskExecutionResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task publish conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task publish request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func startTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/start",
		OperationID: "startTask",
		Summary:     "Start one task by enqueueing executable work",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.TaskExecutionRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TaskExecutionResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task start conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task start request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func cancelTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/cancel",
		OperationID: "cancelTask",
		Summary:     "Cancel one task tree",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.CancelTaskRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task cancel conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task cancel request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func pauseTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDPausePath,
		OperationID: "pauseTask",
		Summary:     "Pause one task for future scheduler claims",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.PauseTaskRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task pause conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task pause request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func resumeTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDResumePath,
		OperationID: "resumeTask",
		Summary:     "Resume one paused task for future scheduler claims",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody:         contract.ResumeTaskRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task resume conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task resume request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createChildTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/children",
		OperationID: "createChildTask",
		Summary:     "Create one child task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Parent task id"),
		},
		RequestBody: contract.CreateTaskChildRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TaskResponse{}},
			{Status: 404, Description: "Task or workspace not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Child task conflict", Body: contract.ErrorPayload{}},
			{Status: 413, Description: specPayloadTooLargeDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid child task request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func addTaskDependencyOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tasks/{id}/dependencies",
		OperationID: "addTaskDependency",
		Summary:     "Add one task dependency",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.AddTaskDependencyRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskDetailResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Dependency conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid dependency request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func removeTaskDependencyOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/tasks/{id}/dependencies/{depends_on_id}",
		OperationID: "removeTaskDependency",
		Summary:     "Remove one task dependency",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			pathParam("depends_on_id", "Dependency task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskDetailResponse{}},
			{Status: 404, Description: "Task or dependency not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid dependency request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listTaskRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksIDRunsPath,
		OperationID: "listTaskRuns",
		Summary:     "List runs for one task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			enumQueryParam("status", "Filter by run status", taskRunStatusValues()),
			queryParam("session_id", "Filter by attached session id", false),
			queryParam("participation_channel", "Filter by resolved participation channel", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunsResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run filter", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func enqueueTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDRunsPath,
		OperationID: "enqueueTaskRun",
		Summary:     "Enqueue one task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.EnqueueTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TaskRunResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run enqueue conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run enqueue request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskAdmissionUnavailableDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/task-runs/{id}",
		OperationID: "getTaskRun",
		Summary:     "Get one task run detail",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunDetailResponse{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run id", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
