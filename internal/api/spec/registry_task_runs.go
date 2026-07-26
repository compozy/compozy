package spec

import "github.com/compozy/agh/internal/api/contract"

func registryTaskRunOperations() []OperationSpec {
	return []OperationSpec{
		inspectRunOperationSpec(),
		fanOutTaskRunsOperationSpec(),
		forceReleaseTaskRunOperationSpec(),
		forceFailTaskRunOperationSpec(),
		retryTaskRunOperationSpec(),
		recoverTaskRunOperationSpec(),
		bulkForceReleaseTaskRunsOperationSpec(),
		bulkForceFailTaskRunsOperationSpec(),
		getSchedulerOperationSpec(),
		pauseSchedulerOperationSpec(),
		resumeSchedulerOperationSpec(),
		drainSchedulerOperationSpec(),
		getSchedulerBacklogOperationSpec(),
		requestTaskRunReviewOperationSpec(),
		listTaskRunReviewsOperationSpec(),
		getTaskRunReviewOperationSpec(),
		submitTaskRunReviewVerdictOperationSpec(),
	}
}
func inspectRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIRunsIDInspectPath,
		OperationID: "inspectRun",
		Summary:     "Inspect one task run with diagnostics",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskInspectResponse{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task-run id", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func fanOutTaskRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDRunsFanOutPath,
		OperationID: "fanOutTaskRuns",
		Summary:     "Enqueue designated sibling runs for one task",
		Tags:        []string{specTasksKey, specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.FanOutTaskRunsRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.FanOutTaskRunsResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task fan-out conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid fan-out request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func forceReleaseTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIRunsIDReleasePath,
		OperationID: "forceReleaseTaskRun",
		Summary:     "Force release one claimed task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.ForceReleaseTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run force-release conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid force-release request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func forceFailTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIRunsIDFailPath,
		OperationID: "forceFailTaskRun",
		Summary:     "Force fail one queued or claimed task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.ForceFailTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run forced-failure conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid forced-failure request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func retryTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIRunsIDRetryPath,
		OperationID: "retryTaskRun",
		Summary:     "Retry one failed task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.RetryTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.RetryTaskRunResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run retry conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid retry request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func recoverTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIRunsIDRecoverPath,
		OperationID: "recoverTaskRun",
		Summary:     "Recover one needs_attention task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.RecoverTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.RetryTaskRunResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run recovery conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid recovery request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func bulkForceReleaseTaskRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIRunsBulkReleasePath,
		OperationID: "bulkForceReleaseTaskRuns",
		Summary:     "Force release a bounded set of claimed task runs",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.BulkForceTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BulkForceTaskRunResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid bulk force-release request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func bulkForceFailTaskRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIRunsBulkFailPath,
		OperationID: "bulkForceFailTaskRuns",
		Summary:     "Force fail a bounded set of queued or claimed task runs",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.BulkForceTaskRunRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BulkForceTaskRunResponse{}},
			{Status: 403, Description: specForceOperationForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid bulk forced-failure request", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specForceOperationRateLimitExceededDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSchedulerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISchedulerPath,
		OperationID: "getScheduler",
		Summary:     "Get scheduler pause state and queue pressure",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SchedulerStatusResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func pauseSchedulerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:              httpMethodPost,
		Path:                specAPISchedulerPausePath,
		OperationID:         "pauseScheduler",
		Summary:             "Pause scheduler dispatch and task-run claims",
		Tags:                []string{specTasksKey},
		Transports:          []Transport{TransportHTTP, TransportUDS},
		RequestBody:         contract.SchedulerPauseRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SchedulerStatusResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid scheduler pause request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func resumeSchedulerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:              httpMethodPost,
		Path:                specAPISchedulerResumePath,
		OperationID:         "resumeScheduler",
		Summary:             "Resume scheduler dispatch and task-run claims",
		Tags:                []string{specTasksKey},
		Transports:          []Transport{TransportHTTP, TransportUDS},
		RequestBody:         contract.SchedulerResumeRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SchedulerStatusResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid scheduler resume request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func drainSchedulerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:              httpMethodPost,
		Path:                specAPISchedulerDrainPath,
		OperationID:         "drainScheduler",
		Summary:             "Pause the scheduler and wait for active claims to drain",
		Tags:                []string{specTasksKey},
		Transports:          []Transport{TransportHTTP, TransportUDS},
		RequestBody:         contract.SchedulerDrainRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SchedulerDrainResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid scheduler drain request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSchedulerBacklogOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISchedulerBacklogPath,
		OperationID: "getSchedulerBacklog",
		Summary:     "List queued scheduler backlog rows",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			intQueryParam("limit", "Maximum number of queued runs to return"),
			enumQueryParam(specScopeKey, "Filter by catalog visibility", taskCatalogScopeValues()),
			queryParam(specWorkspaceKey, "Filter by workspace path, name, or ID", false),
			boolQueryParam("include_paused", "Include runs blocked by task pause state"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SchedulerBacklogResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid scheduler backlog query", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func requestTaskRunReviewOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITaskRunsIDReviewsPath,
		OperationID: "requestTaskRunReview",
		Summary:     "Request review for one terminal task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
		},
		RequestBody: contract.CreateTaskRunReviewRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TaskRunReviewRequestResponse{}},
			{Status: 200, Description: "OK", Body: contract.TaskRunReviewRequestResponse{}},
			{Status: 400, Description: "Invalid review request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Review request conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listTaskRunReviewsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITaskRunsIDReviewsPath,
		OperationID: "listTaskRunReviews",
		Summary:     "List reviews for one task run",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task run id"),
			enumQueryParam("status", "Filter by review status", taskRunReviewStatusValues()),
			queryParam("reviewer_session_id", "Filter by reviewer session id", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunReviewsResponse{}},
			{Status: 400, Description: "Invalid review filter", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskRunReviewOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/task-reviews/{id}",
		OperationID: "getTaskRunReview",
		Summary:     "Get one task-run review",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Review id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunReviewResponse{}},
			{Status: 404, Description: "Review not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func submitTaskRunReviewVerdictOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/task-reviews/{id}/verdict",
		OperationID: "submitTaskRunReviewVerdict",
		Summary:     "Submit one task-run review verdict",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Review id"),
		},
		RequestBody: contract.SubmitTaskRunReviewVerdictRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunReviewVerdictResponse{}},
			{Status: 400, Description: "Invalid review verdict", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Review or task run not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Review verdict conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
