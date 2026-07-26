package spec

import "github.com/compozy/agh/internal/api/contract"

func registryAgentRuntimeOperations() []OperationSpec {
	return []OperationSpec{
		getAgentMeOperationSpec(),
		getAgentContextOperationSpec(),
		listAgentChannelsOperationSpec(),
		receiveAgentChannelMessagesOperationSpec(),
		sendAgentChannelMessageOperationSpec(),
		replyAgentChannelMessageOperationSpec(),
		claimNextAgentTaskOperationSpec(),
		heartbeatAgentTaskRunOperationSpec(),
		completeAgentTaskRunOperationSpec(),
		failAgentTaskRunOperationSpec(),
		releaseAgentTaskRunOperationSpec(),
		spawnAgentSessionOperationSpec(),
		getAgentCoordinatorConfigOperationSpec(),
	}
}
func getAgentMeOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/agent/me",
		OperationID: "getAgentMe",
		Summary:     "Resolve the calling agent session",
		Tags:        []string{specAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentMeResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Caller session not found", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getAgentContextOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/agent/context",
		OperationID: "getAgentContext",
		Summary:     "Return the bounded calling-agent situation context",
		Tags:        []string{specAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentContextResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Caller session not found", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listAgentChannelsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/agent/channels",
		OperationID: "listAgentChannels",
		Summary:     "List coordination channels visible to the calling agent",
		Tags:        []string{specAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentChannelsResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func receiveAgentChannelMessagesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/agent/channels/{channel}/recv",
		OperationID: "receiveAgentChannelMessages",
		Summary:     "Receive task-bound coordination channel messages",
		Tags:        []string{specAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("channel", "Coordination channel id"),
			boolQueryParam("wait", "Wait for the next message when no messages are immediately available"),
			intQueryParam("limit", "Maximum number of messages to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentChannelMessagesResponse{}},
			{Status: 400, Description: "Invalid channel receive query", Body: contract.ErrorPayload{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Coordination channel not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid channel receive request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func sendAgentChannelMessageOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/channels/{channel}/send",
		OperationID: "sendAgentChannelMessage",
		Summary:     "Send one task-bound coordination channel message",
		Tags:        []string{specAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("channel", "Coordination channel id"),
		},
		RequestBody: contract.AgentChannelSendRequest{},
		Responses: []ResponseSpec{
			{Status: 202, Description: specAcceptedDescription, Body: contract.AgentChannelMessageResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Coordination channel not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid channel send request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func replyAgentChannelMessageOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/channels/reply",
		OperationID: "replyAgentChannelMessage",
		Summary:     "Reply to one delivered coordination channel message",
		Tags:        []string{specAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.AgentChannelReplyRequest{},
		Responses: []ResponseSpec{
			{Status: 202, Description: specAcceptedDescription, Body: contract.AgentChannelMessageResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Coordination message not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid channel reply request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func claimNextAgentTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAgentTaskClaimNextPath,
		OperationID: "claimNextAgentTask",
		Summary:     "Atomically claim the next matching task run for the calling agent",
		Tags:        []string{specAgentKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.AgentTaskClaimNextRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentTaskClaimResponse{}},
			{Status: 204, Description: "No matching task run is currently claimable"},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 409, Description: "Task-run claim conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid claim criteria", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specTaskAdmissionUnavailableDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func heartbeatAgentTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/tasks/{run_id}/heartbeat",
		OperationID: "heartbeatAgentTaskRun",
		Summary:     "Extend a claimed task-run lease for the calling agent",
		Tags:        []string{specAgentKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("run_id", "Task run id"),
		},
		RequestBody: contract.AgentTaskHeartbeatRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentTaskLeaseResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run lease conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid heartbeat request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func completeAgentTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/tasks/{run_id}/complete",
		OperationID: "completeAgentTaskRun",
		Summary:     "Complete a claimed task run for the calling agent",
		Tags:        []string{specAgentKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("run_id", "Task run id"),
		},
		RequestBody: contract.AgentTaskCompleteRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentTaskLeaseResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run completion conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid completion request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func failAgentTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/tasks/{run_id}/fail",
		OperationID: "failAgentTaskRun",
		Summary:     "Fail a claimed task run for the calling agent",
		Tags:        []string{specAgentKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("run_id", "Task run id"),
		},
		RequestBody: contract.AgentTaskFailRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentTaskLeaseResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run failure conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid failure request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func releaseAgentTaskRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/tasks/{run_id}/release",
		OperationID: "releaseAgentTaskRun",
		Summary:     "Release a claimed task run for the calling agent",
		Tags:        []string{specAgentKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("run_id", "Task run id"),
		},
		RequestBody: contract.AgentTaskReleaseRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentTaskLeaseResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specTaskRunNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Task-run release conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid release request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func spawnAgentSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/agent/spawn",
		OperationID: "spawnAgentSession",
		Summary:     "Spawn a narrowed child session for the calling agent",
		Tags:        []string{specAgentKey, specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.AgentSpawnRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.AgentSpawnResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: "Spawn permission denied", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Spawn limit conflict", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid spawn request", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getAgentCoordinatorConfigOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/agent/coordinator/config",
		OperationID: "getAgentCoordinatorConfig",
		Summary:     "Read resolved coordinator config for the calling agent workspace",
		Tags:        []string{specAgentKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam(specWorkspaceKey, "Workspace id or path", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentCoordinatorConfigResponse{}},
			{Status: 401, Description: specAgentCallerIdentityIsMissingDescription, Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: specForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
