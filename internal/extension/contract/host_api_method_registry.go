package contract

import (
	apicontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

var hostAPIMethodSpecs = []HostAPIMethodSpec{
	{
		Method:         HostAPIMethodSessionsList,
		Params:         NamedType{Name: "SessionsListParams", Value: SessionsListParams{}},
		Result:         NamedType{Name: "SessionSummary", Value: []SessionSummary{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodSessionsCreate,
		Params: NamedType{Name: "SessionsCreateParams", Value: SessionsCreateParams{}},
		Result: NamedType{Name: "SessionCreateResult", Value: SessionCreateResult{}},
	},
	{
		Method: HostAPIMethodSessionsPrompt,
		Params: NamedType{Name: "SessionsPromptParams", Value: SessionsPromptParams{}},
		Result: NamedType{Name: "SessionPromptResult", Value: SessionPromptResult{}},
	},
	{
		Method: HostAPIMethodSessionsStop,
		Params: NamedType{Name: "SessionTargetParams", Value: SessionTargetParams{}},
		Result: NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
	},
	{
		Method: HostAPIMethodSessionsStatus,
		Params: NamedType{Name: "SessionTargetParams", Value: SessionTargetParams{}},
		Result: NamedType{Name: "SessionStatus", Value: SessionStatus{}},
	},
	{
		Method: HostAPIMethodSessionsEvents,
		Params: NamedType{Name: "SessionEventsParams", Value: SessionEventsParams{}},
		Result: NamedType{Name: "SessionEvent", Value: []SessionEvent{}},
	},
	{
		Method: HostAPIMethodSessionsSoulRefresh,
		Params: NamedType{Name: "SessionSoulRefreshParams", Value: SessionSoulRefreshParams{}},
		Result: NamedType{Name: hostAPIAgentSoulPayloadValue, Value: apicontract.AgentSoulPayload{}},
	},
	{
		Method: HostAPIMethodSessionsHealthGet,
		Params: NamedType{Name: "SessionHealthGetParams", Value: SessionHealthGetParams{}},
		Result: NamedType{Name: "SessionHealthResponse", Value: apicontract.SessionHealthResponse{}},
	},
	{
		Method: HostAPIMethodSessionsStatusGet,
		Params: NamedType{Name: "SessionStatusGetParams", Value: SessionStatusGetParams{}},
		Result: NamedType{Name: "SessionStatusResponse", Value: apicontract.SessionStatusResponse{}},
	},
	{
		Method:         HostAPIMethodSandboxList,
		Params:         NamedType{Name: "SandboxListParams", Value: SandboxListParams{}},
		Result:         NamedType{Name: "SandboxListResult", Value: SandboxListResult{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodSandboxInfo,
		Params: NamedType{Name: "SandboxInfoParams", Value: SandboxInfoParams{}},
		Result: NamedType{Name: "SandboxInfoResult", Value: SandboxInfoResult{}},
	},
	{
		Method: HostAPIMethodSandboxExec,
		Params: NamedType{Name: "SandboxExecParams", Value: SandboxExecParams{}},
		Result: NamedType{Name: "SandboxExecResult", Value: SandboxExecResult{}},
	},
	{
		Method: HostAPIMethodMemoryRecall,
		Params: NamedType{Name: "MemoryRecallParams", Value: MemoryRecallParams{}},
		Result: NamedType{Name: "MemoryRecallEntry", Value: []MemoryRecallEntry{}},
	},
	{
		Method: HostAPIMethodMemoryStore,
		Params: NamedType{Name: "MemoryStoreParams", Value: MemoryStoreParams{}},
		Result: NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
	},
	{
		Method: HostAPIMethodMemoryForget,
		Params: NamedType{Name: "MemoryForgetParams", Value: MemoryForgetParams{}},
		Result: NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
	},
	{
		Method:         HostAPIMethodObserveHealth,
		Params:         NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
		Result:         NamedType{Name: "ObserveHealth", Value: ObserveHealth{}},
		OptionalParams: true,
	},
	{
		Method:         HostAPIMethodListLogs,
		Params:         NamedType{Name: "ListLogsParams", Value: ListLogsParams{}},
		Result:         NamedType{Name: "SessionEvent", Value: []SessionEvent{}},
		OptionalParams: true,
	},
	{
		Method:         HostAPIMethodSkillsList,
		Params:         NamedType{Name: "SkillsListParams", Value: SkillsListParams{}},
		Result:         NamedType{Name: "SkillSummary", Value: []SkillSummary{}},
		OptionalParams: true,
	},
	{
		Method:         HostAPIMethodModelsList,
		Params:         NamedType{Name: "ModelsListParams", Value: ModelsListParams{}},
		Result:         NamedType{Name: "ProviderModelListResponse", Value: apicontract.ProviderModelListResponse{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodModelsRefresh,
		Params: NamedType{Name: "ModelsRefreshParams", Value: ModelsRefreshParams{}},
		Result: NamedType{
			Name:  "ProviderModelRefreshResponse",
			Value: apicontract.ProviderModelRefreshResponse{},
		},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodModelsStatus,
		Params: NamedType{Name: "ModelsStatusParams", Value: ModelsStatusParams{}},
		Result: NamedType{
			Name:  "ProviderModelStatusResponse",
			Value: apicontract.ProviderModelStatusResponse{},
		},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodAgentsSoulGet,
		Params: NamedType{Name: "AgentSoulGetParams", Value: AgentSoulGetParams{}},
		Result: NamedType{Name: hostAPIAgentSoulPayloadValue, Value: apicontract.AgentSoulPayload{}},
	},
	{
		Method: HostAPIMethodAgentsSoulValidate,
		Params: NamedType{Name: "AgentSoulValidateParams", Value: AgentSoulValidateParams{}},
		Result: NamedType{Name: hostAPIAgentSoulPayloadValue, Value: apicontract.AgentSoulPayload{}},
	},
	{
		Method: HostAPIMethodAgentsSoulPut,
		Params: NamedType{Name: "AgentSoulPutParams", Value: AgentSoulPutParams{}},
		Result: NamedType{Name: hostAPIAgentSoulMutationResponseValue, Value: apicontract.AgentSoulMutationResponse{}},
	},
	{
		Method: HostAPIMethodAgentsSoulDelete,
		Params: NamedType{Name: "AgentSoulDeleteParams", Value: AgentSoulDeleteParams{}},
		Result: NamedType{Name: hostAPIAgentSoulMutationResponseValue, Value: apicontract.AgentSoulMutationResponse{}},
	},
	{
		Method: HostAPIMethodAgentsSoulHistory,
		Params: NamedType{Name: "AgentSoulHistoryParams", Value: AgentSoulHistoryParams{}},
		Result: NamedType{Name: "AgentSoulHistoryResponse", Value: apicontract.AgentSoulHistoryResponse{}},
	},
	{
		Method: HostAPIMethodAgentsSoulRollback,
		Params: NamedType{Name: "AgentSoulRollbackParams", Value: AgentSoulRollbackParams{}},
		Result: NamedType{Name: hostAPIAgentSoulMutationResponseValue, Value: apicontract.AgentSoulMutationResponse{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatGet,
		Params: NamedType{Name: "AgentHeartbeatGetParams", Value: AgentHeartbeatGetParams{}},
		Result: NamedType{Name: hostAPIHeartbeatPolicyPayloadValue, Value: apicontract.HeartbeatPolicyPayload{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatValidate,
		Params: NamedType{Name: "AgentHeartbeatValidateParams", Value: AgentHeartbeatValidateParams{}},
		Result: NamedType{Name: hostAPIHeartbeatPolicyPayloadValue, Value: apicontract.HeartbeatPolicyPayload{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatPut,
		Params: NamedType{Name: "AgentHeartbeatPutParams", Value: AgentHeartbeatPutParams{}},
		Result: NamedType{Name: hostAPIHeartbeatMutationResponseValue, Value: apicontract.HeartbeatMutationResponse{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatDelete,
		Params: NamedType{Name: "AgentHeartbeatDeleteParams", Value: AgentHeartbeatDeleteParams{}},
		Result: NamedType{Name: hostAPIHeartbeatMutationResponseValue, Value: apicontract.HeartbeatMutationResponse{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatHistory,
		Params: NamedType{Name: "AgentHeartbeatHistoryParams", Value: AgentHeartbeatHistoryParams{}},
		Result: NamedType{Name: "HeartbeatHistoryResponse", Value: apicontract.HeartbeatHistoryResponse{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatRollback,
		Params: NamedType{Name: "AgentHeartbeatRollbackParams", Value: AgentHeartbeatRollbackParams{}},
		Result: NamedType{Name: hostAPIHeartbeatMutationResponseValue, Value: apicontract.HeartbeatMutationResponse{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatStatus,
		Params: NamedType{Name: "AgentHeartbeatStatusParams", Value: AgentHeartbeatStatusParams{}},
		Result: NamedType{Name: "HeartbeatStatusResponse", Value: apicontract.HeartbeatStatusResponse{}},
	},
	{
		Method: HostAPIMethodAgentsHeartbeatWake,
		Params: NamedType{Name: "AgentHeartbeatWakeParams", Value: AgentHeartbeatWakeParams{}},
		Result: NamedType{Name: "HeartbeatWakeResponse", Value: apicontract.HeartbeatWakeResponse{}},
	},
	{
		Method:         HostAPIMethodAutomationJobs,
		Params:         NamedType{Name: "AutomationJobsParams", Value: AutomationJobsParams{}},
		Result:         NamedType{Name: "AutomationJobsResult", Value: AutomationJobsResult{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodAutomationJobsGet,
		Params: NamedType{Name: hostAPIAutomationTargetParamsValue, Value: AutomationTargetParams{}},
		Result: NamedType{Name: hostAPIJobValue, Value: automationpkg.Job{}},
	},
	{
		Method: HostAPIMethodAutomationJobsCreate,
		Params: NamedType{Name: "AutomationJobCreateParams", Value: AutomationJobCreateParams{}},
		Result: NamedType{Name: hostAPIJobValue, Value: automationpkg.Job{}},
	},
	{
		Method: HostAPIMethodAutomationJobsUpdate,
		Params: NamedType{Name: "AutomationJobUpdateParams", Value: AutomationJobUpdateParams{}},
		Result: NamedType{Name: hostAPIJobValue, Value: automationpkg.Job{}},
	},
	{
		Method: HostAPIMethodAutomationJobsDelete,
		Params: NamedType{Name: hostAPIAutomationTargetParamsValue, Value: AutomationTargetParams{}},
		Result: NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
	},
	{
		Method: HostAPIMethodAutomationJobsTrigger,
		Params: NamedType{Name: "AutomationJobTriggerParams", Value: AutomationJobTriggerParams{}},
		Result: NamedType{Name: hostAPIRunValue, Value: automationpkg.Run{}},
	},
	{
		Method: HostAPIMethodAutomationJobsRuns,
		Params: NamedType{Name: "AutomationJobRunsParams", Value: AutomationJobRunsParams{}},
		Result: NamedType{Name: hostAPIRunValue, Value: []automationpkg.Run{}},
	},
	{
		Method:         HostAPIMethodAutomationTriggers,
		Params:         NamedType{Name: "AutomationTriggersParams", Value: AutomationTriggersParams{}},
		Result:         NamedType{Name: "AutomationTriggersResult", Value: AutomationTriggersResult{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodAutomationTriggersGet,
		Params: NamedType{Name: hostAPIAutomationTargetParamsValue, Value: AutomationTargetParams{}},
		Result: NamedType{Name: hostAPITriggerValue, Value: apicontract.TriggerPayload{}},
	},
	{
		Method: HostAPIMethodAutomationTriggersCreate,
		Params: NamedType{Name: "AutomationTriggerCreateParams", Value: AutomationTriggerCreateParams{}},
		Result: NamedType{Name: hostAPITriggerValue, Value: apicontract.TriggerPayload{}},
	},
	{
		Method: HostAPIMethodAutomationTriggersUpdate,
		Params: NamedType{Name: "AutomationTriggerUpdateParams", Value: AutomationTriggerUpdateParams{}},
		Result: NamedType{Name: hostAPITriggerValue, Value: apicontract.TriggerPayload{}},
	},
	{
		Method: HostAPIMethodAutomationTriggersDelete,
		Params: NamedType{Name: hostAPIAutomationTargetParamsValue, Value: AutomationTargetParams{}},
		Result: NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
	},
	{
		Method: HostAPIMethodAutomationTriggersRuns,
		Params: NamedType{Name: "AutomationTriggerRunsParams", Value: AutomationTriggerRunsParams{}},
		Result: NamedType{Name: hostAPIRunValue, Value: []automationpkg.Run{}},
	},
	{
		Method: HostAPIMethodAutomationTriggersFire,
		Params: NamedType{Name: "AutomationTriggerFireParams", Value: AutomationTriggerFireParams{}},
		Result: NamedType{Name: "TriggerResult", Value: automationpkg.TriggerResult{}},
	},
	{
		Method:         HostAPIMethodAutomationRuns,
		Params:         NamedType{Name: "AutomationRunsParams", Value: AutomationRunsParams{}},
		Result:         NamedType{Name: hostAPIRunValue, Value: []automationpkg.Run{}},
		OptionalParams: true,
	},
	{
		Method:         HostAPIMethodTasks,
		Params:         NamedType{Name: "TasksParams", Value: TasksParams{}},
		Result:         NamedType{Name: "TasksResponse", Value: apicontract.TasksResponse{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodTasksGet,
		Params: NamedType{Name: "TaskTargetParams", Value: TaskTargetParams{}},
		Result: NamedType{Name: "TaskDetail", Value: apicontract.TaskDetailPayload{}},
	},
	{
		Method: HostAPIMethodTasksTimeline,
		Params: NamedType{Name: "TaskTimelineParams", Value: TaskTimelineParams{}},
		Result: NamedType{Name: "TaskTimelineItem", Value: []apicontract.TaskTimelineItemPayload{}},
	},
	{
		Method: HostAPIMethodTasksTree,
		Params: NamedType{Name: "TaskTreeParams", Value: TaskTreeParams{}},
		Result: NamedType{Name: "TaskTree", Value: apicontract.TaskTreePayload{}},
	},
	{
		Method:         HostAPIMethodTasksDashboard,
		Params:         NamedType{Name: "TaskDashboardParams", Value: TaskDashboardParams{}},
		Result:         NamedType{Name: "TaskDashboard", Value: apicontract.TaskDashboardPayload{}},
		OptionalParams: true,
	},
	{
		Method:         HostAPIMethodTasksInbox,
		Params:         NamedType{Name: "TaskInboxParams", Value: TaskInboxParams{}},
		Result:         NamedType{Name: "TaskInbox", Value: apicontract.TaskInboxPayload{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodTasksCreate,
		Params: NamedType{Name: "TaskCreateParams", Value: TaskCreateParams{}},
		Result: NamedType{Name: hostAPITaskValue, Value: apicontract.TaskPayload{}},
	},
	{
		Method: HostAPIMethodTasksUpdate,
		Params: NamedType{Name: "TaskUpdateParams", Value: TaskUpdateParams{}},
		Result: NamedType{Name: hostAPITaskValue, Value: apicontract.TaskPayload{}},
	},
	{
		Method: HostAPIMethodTasksCancel,
		Params: NamedType{Name: "TaskCancelParams", Value: TaskCancelParams{}},
		Result: NamedType{Name: hostAPITaskValue, Value: apicontract.TaskPayload{}},
	},
	{
		Method: HostAPIMethodTasksRuns,
		Params: NamedType{Name: "TaskRunsParams", Value: TaskRunsParams{}},
		Result: NamedType{Name: hostAPITaskRunValue, Value: []apicontract.TaskRunPayload{}},
	},
	{
		Method: HostAPIMethodTasksRunsGet,
		Params: NamedType{Name: "TaskRunGetParams", Value: TaskRunGetParams{}},
		Result: NamedType{Name: "TaskRunDetail", Value: apicontract.TaskRunDetailPayload{}},
	},
	{
		Method: HostAPIMethodTasksRunsEnqueue,
		Params: NamedType{Name: "TaskRunEnqueueParams", Value: TaskRunEnqueueParams{}},
		Result: NamedType{Name: hostAPITaskRunValue, Value: apicontract.TaskRunPayload{}},
	},
	{
		Method: HostAPIMethodTasksRunsStart,
		Params: NamedType{Name: "TaskRunStartParams", Value: TaskRunStartParams{}},
		Result: NamedType{Name: hostAPITaskRunValue, Value: apicontract.TaskRunPayload{}},
	},
	{
		Method: HostAPIMethodTasksRunsAttachSession,
		Params: NamedType{Name: "TaskRunAttachSessionParams", Value: TaskRunAttachSessionParams{}},
		Result: NamedType{Name: hostAPITaskRunValue, Value: apicontract.TaskRunPayload{}},
	},
	{
		Method: HostAPIMethodTasksRunsComplete,
		Params: NamedType{Name: "TaskRunCompleteParams", Value: TaskRunCompleteParams{}},
		Result: NamedType{Name: hostAPITaskRunValue, Value: apicontract.TaskRunPayload{}},
	},
	{
		Method: HostAPIMethodTasksRunsFail,
		Params: NamedType{Name: "TaskRunFailParams", Value: TaskRunFailParams{}},
		Result: NamedType{Name: hostAPITaskRunValue, Value: apicontract.TaskRunPayload{}},
	},
	{
		Method: HostAPIMethodTasksRunsCancel,
		Params: NamedType{Name: "TaskRunCancelParams", Value: TaskRunCancelParams{}},
		Result: NamedType{Name: hostAPITaskRunValue, Value: apicontract.TaskRunPayload{}},
	},
	{
		Method:         HostAPIMethodNetworkStatus,
		Params:         NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
		Result:         NamedType{Name: "NetworkStatusPayload", Value: apicontract.NetworkStatusPayload{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodNetworkUsage,
		Params: NamedType{Name: "NetworkUsageParams", Value: NetworkUsageParams{}},
		Result: NamedType{Name: "NetworkUsageResponse", Value: apicontract.NetworkUsageResponse{}},
	},
	{
		Method: HostAPIMethodNetworkChannels,
		Params: NamedType{Name: "NetworkChannelsParams", Value: NetworkChannelsParams{}},
		Result: NamedType{Name: "NetworkChannelPayload", Value: []apicontract.NetworkChannelPayload{}},
	},
	{
		Method: HostAPIMethodNetworkPeers,
		Params: NamedType{Name: "NetworkPeersParams", Value: NetworkPeersParams{}},
		Result: NamedType{Name: "NetworkPeerPayload", Value: []apicontract.NetworkPeerPayload{}},
	},
	{
		Method: HostAPIMethodNetworkThreads,
		Params: NamedType{Name: "NetworkThreadsParams", Value: NetworkThreadsParams{}},
		Result: NamedType{Name: "NetworkThreadsResponse", Value: apicontract.NetworkThreadsResponse{}},
	},
	{
		Method: HostAPIMethodNetworkThreadGet,
		Params: NamedType{Name: "NetworkThreadTargetParams", Value: NetworkThreadTargetParams{}},
		Result: NamedType{Name: "NetworkThreadSummaryPayload", Value: apicontract.NetworkThreadSummaryPayload{}},
	},
	{
		Method: HostAPIMethodNetworkThreadMessages,
		Params: NamedType{Name: "NetworkThreadMessagesParams", Value: NetworkThreadMessagesParams{}},
		Result: NamedType{
			Name:  "NetworkThreadMessagesResponse",
			Value: apicontract.NetworkThreadMessagesResponse{},
		},
	},
	{
		Method: HostAPIMethodNetworkDirects,
		Params: NamedType{Name: "NetworkDirectsParams", Value: NetworkDirectsParams{}},
		Result: NamedType{Name: "NetworkDirectRoomsResponse", Value: apicontract.NetworkDirectRoomsResponse{}},
	},
	{
		Method: HostAPIMethodNetworkDirectResolve,
		Params: NamedType{Name: "NetworkDirectResolveParams", Value: NetworkDirectResolveParams{}},
		Result: NamedType{Name: "NetworkDirectRoomPayload", Value: apicontract.NetworkDirectRoomPayload{}},
	},
	{
		Method: HostAPIMethodNetworkDirectMessages,
		Params: NamedType{Name: "NetworkDirectMessagesParams", Value: NetworkDirectMessagesParams{}},
		Result: NamedType{
			Name:  "NetworkDirectRoomMessagesResponse",
			Value: apicontract.NetworkDirectRoomMessagesResponse{},
		},
	},
	{
		Method: HostAPIMethodNetworkWorkGet,
		Params: NamedType{Name: "NetworkWorkGetParams", Value: NetworkWorkGetParams{}},
		Result: NamedType{Name: "NetworkWorkPayload", Value: apicontract.NetworkWorkPayload{}},
	},
	{
		Method: HostAPIMethodNetworkSend,
		Params: NamedType{Name: "NetworkSendParams", Value: NetworkSendParams{}},
		Result: NamedType{Name: "NetworkSendPayload", Value: apicontract.NetworkSendPayload{}},
	},
	{
		Method:         HostAPIMethodResourcesList,
		Params:         NamedType{Name: "ResourcesListParams", Value: ResourcesListParams{}},
		Result:         NamedType{Name: hostAPIResourceRecordValue, Value: []ResourceRecord{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodResourcesGet,
		Params: NamedType{Name: "ResourceGetParams", Value: ResourceGetParams{}},
		Result: NamedType{Name: hostAPIResourceRecordValue, Value: ResourceRecord{}},
	},
	{
		Method: HostAPIMethodResourcesSnapshot,
		Params: NamedType{Name: "ResourcesSnapshotParams", Value: ResourcesSnapshotParams{}},
		Result: NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
	},
	{
		Method:         HostAPIMethodBridgesInstancesList,
		Params:         NamedType{Name: hostAPIEmptyResultValue, Value: EmptyResult{}},
		Result:         NamedType{Name: hostAPIBridgeInstanceValue, Value: []bridgepkg.BridgeInstance{}},
		OptionalParams: true,
	},
	{
		Method: HostAPIMethodBridgesMessagesIngest,
		Params: NamedType{Name: "InboundMessageEnvelope", Value: bridgepkg.InboundMessageEnvelope{}},
		Result: NamedType{Name: "BridgesMessagesIngestResult", Value: BridgesMessagesIngestResult{}},
	},
	{
		Method: HostAPIMethodBridgesInstancesGet,
		Params: NamedType{Name: "BridgeInstanceTargetParams", Value: BridgeInstanceTargetParams{}},
		Result: NamedType{Name: hostAPIBridgeInstanceValue, Value: bridgepkg.BridgeInstance{}},
	},
	{
		Method: HostAPIMethodBridgesInstancesReportState,
		Params: NamedType{Name: "BridgesInstancesReportStateParams", Value: BridgesInstancesReportStateParams{}},
		Result: NamedType{Name: hostAPIBridgeInstanceValue, Value: bridgepkg.BridgeInstance{}},
	},
}

// HostAPIMethodSpecs returns the canonical Host API method registry in wire order.
func HostAPIMethodSpecs() []HostAPIMethodSpec {
	return append(append([]HostAPIMethodSpec(nil), hostAPIMethodSpecs...), clarifyHostAPIMethodSpec())
}
