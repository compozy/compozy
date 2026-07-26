package spec

import "github.com/compozy/agh/internal/api/contract"

func registryTaskManagementOperations() []OperationSpec {
	return []OperationSpec{
		listTasksOperationSpec(),
		createTaskOperationSpec(),
		getTaskOperationSpec(),
		inspectTaskOperationSpec(),
		deleteTaskOperationSpec(),
		updateTaskOperationSpec(),
		blockTaskOperationSpec(),
		listTaskBlocksOperationSpec(),
		clearTaskBlockOperationSpec(),
		recoverTaskOperationSpec(),
		getTaskExecutionProfileOperationSpec(),
		setTaskExecutionProfileOperationSpec(),
		deleteTaskExecutionProfileOperationSpec(),
		createTaskBridgeNotificationSubscriptionOperationSpec(),
		listTaskBridgeNotificationSubscriptionsOperationSpec(),
		deleteTaskBridgeNotificationSubscriptionOperationSpec(),
		getTaskBridgeNotificationSubscriptionOperationSpec(),
		listTaskReviewsOperationSpec(),
	}
}
func listTasksOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksPath,
		OperationID: "listTasks",
		Summary:     "List a bounded task catalog page",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Filter by catalog visibility", taskCatalogScopeValues()),
			queryParam(specWorkspaceKey, "Filter by workspace path, name, or ID", false),
			enumQueryParam("status", "Filter by task status", taskStatusValues()),
			enumQueryParam("priority", "Filter by task priority", taskPriorityValues()),
			boolQueryParam("include_drafts", "Include draft tasks in list results"),
			enumQueryParam("approval_state", "Filter by task approval state", taskApprovalStateValues()),
			enumQueryParam("owner_kind", "Filter by owner kind", taskOwnerKindValues()),
			queryParam("owner_ref", "Filter by owner reference", false),
			queryParam("parent_task_id", "Filter by parent task ID", false),
			queryParam("participation_channel", "Filter by resolved participation channel", false),
			queryParam("query", "Filter by task title or identifier", false),
			enumQueryParam("sort", "Order by recent activity or priority", taskCatalogSortValues()),
			queryParam("cursor", "Opaque query-bound continuation cursor", false),
			intQueryParam("limit", "Page size from 1 to 200 (default 50)"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TasksResponse{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 400, Description: "Invalid task filter or cursor", Body: contract.ErrorPayload{}},
			{Status: 410, Description: workspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksPath,
		OperationID: "createTask",
		Summary:     "Create a task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateTaskRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TaskResponse{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task conflict", Body: contract.ErrorPayload{}},
			{Status: 413, Description: specPayloadTooLargeDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksIDPath,
		OperationID: "getTask",
		Summary:     "Get one task with detail",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskDetailResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskIDDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func inspectTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksIDInspectPath,
		OperationID: "inspectTask",
		Summary:     "Inspect one task with diagnostics",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskInspectResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskIDDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPITasksIDPath,
		OperationID: "deleteTask",
		Summary:     "Delete one task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 400, Description: "Invalid task delete", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPITasksIDPath,
		OperationID: "updateTask",
		Summary:     "Update one task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.UpdateTaskRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task update conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task update", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func blockTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDBlocksPath,
		OperationID: "blockTask",
		Summary:     "Create one typed task block",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.CreateTaskBlockRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TaskBlockResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task block conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task block request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listTaskBlocksOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksIDBlocksPath,
		OperationID: "listTaskBlocks",
		Summary:     "List typed task blocks",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			boolQueryParam("include_cleared", "Include cleared task blocks"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskBlocksResponse{}},
			{Status: 400, Description: "Invalid task block query", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskIDDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func clearTaskBlockOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDBlocksBlockIDClearPath,
		OperationID: "clearTaskBlock",
		Summary:     "Clear one typed task block",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			pathParam("block_id", "Task block id"),
		},
		RequestBody: contract.ClearTaskBlockRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskBlockResponse{}},
			{Status: 404, Description: "Task or block not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task block conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task block clear request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func recoverTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDRecoverPath,
		OperationID: "recoverTask",
		Summary:     "Recover one task from needs_attention",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.RecoverTaskRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task recover conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid task recover request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskExecutionProfileOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksIDExecutionProfilePath,
		OperationID: "getTaskExecutionProfile",
		Summary:     "Get one task execution profile",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskExecutionProfileResponse{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidTaskIDDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func setTaskExecutionProfileOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPITasksIDExecutionProfilePath,
		OperationID: "setTaskExecutionProfile",
		Summary:     "Replace one task execution profile",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.SetTaskExecutionProfileRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskExecutionProfileResponse{}},
			{Status: 400, Description: "Invalid task execution profile", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task execution profile conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteTaskExecutionProfileOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPITasksIDExecutionProfilePath,
		OperationID: "deleteTaskExecutionProfile",
		Summary:     "Delete one task execution profile",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: "Task or execution profile not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task execution profile conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createTaskBridgeNotificationSubscriptionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPITasksIDNotificationsBridgesPath,
		OperationID: "createTaskBridgeNotificationSubscription",
		Summary:     "Create one bridge terminal notification subscription for a task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
		},
		RequestBody: contract.CreateTaskBridgeNotificationSubscriptionRequest{},
		Responses: []ResponseSpec{
			{
				Status:      201,
				Description: specCreatedDescription,
				Body:        contract.TaskBridgeNotificationSubscriptionResponse{},
			},
			{Status: 400, Description: "Invalid bridge notification subscription", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Task or bridge not found", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specTaskOrBridgeServiceIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listTaskBridgeNotificationSubscriptionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksIDNotificationsBridgesPath,
		OperationID: "listTaskBridgeNotificationSubscriptions",
		Summary:     "List bridge terminal notification subscriptions for one task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			queryParam("bridge_instance_id", "Filter by bridge instance id", false),
			enumQueryParam(specScopeKey, "Filter by bridge scope", bridgeScopeValues()),
			queryParam("workspace_id", "Filter by workspace id", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskBridgeNotificationSubscriptionsResponse{}},
			{Status: 400, Description: "Invalid bridge notification filter", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specTaskOrBridgeServiceIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteTaskBridgeNotificationSubscriptionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPITasksIDNotificationsBridgesSubscriptionIDPath,
		OperationID: "deleteTaskBridgeNotificationSubscription",
		Summary:     "Delete one bridge terminal notification subscription for a task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			pathParam("subscription_id", "Bridge task subscription id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{
				Status:      404,
				Description: "Task or bridge notification subscription not found",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      503,
				Description: specTaskOrBridgeServiceIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getTaskBridgeNotificationSubscriptionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPITasksIDNotificationsBridgesSubscriptionIDPath,
		OperationID: "getTaskBridgeNotificationSubscription",
		Summary:     "Get one bridge terminal notification subscription for a task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			pathParam("subscription_id", "Bridge task subscription id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskBridgeNotificationSubscriptionResponse{}},
			{
				Status:      404,
				Description: "Task or bridge notification subscription not found",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      503,
				Description: specTaskOrBridgeServiceIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listTaskReviewsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/tasks/{id}/reviews",
		OperationID: "listTaskReviews",
		Summary:     "List task-run reviews for one task",
		Tags:        []string{specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Task id"),
			enumQueryParam("status", "Filter by review status", taskRunReviewStatusValues()),
			queryParam("reviewer_session_id", "Filter by reviewer session id", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TaskRunReviewsResponse{}},
			{Status: 400, Description: "Invalid review filter", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specTaskNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specTaskServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
