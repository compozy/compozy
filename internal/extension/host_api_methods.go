package extensionpkg

import extensioncontract "github.com/compozy/compozy/internal/extension/contract"

func hostAPIMethodHandlers(handler *HostAPIHandler) map[string]hostAPIMethodFunc {
	handlers := map[string]hostAPIMethodFunc{
		hostAPIAutomationJobsPath:                                      handler.handleAutomationJobs,
		hostAPIAutomationJobsGetPath:                                   handler.handleAutomationJobsGet,
		hostAPIAutomationJobsCreatePath:                                handler.handleAutomationJobsCreate,
		hostAPIAutomationJobsUpdatePath:                                handler.handleAutomationJobsUpdate,
		hostAPIAutomationJobsDeletePath:                                handler.handleAutomationJobsDelete,
		hostAPIAutomationJobsTriggerPath:                               handler.handleAutomationJobsTrigger,
		hostAPIAutomationJobsRunsPath:                                  handler.handleAutomationJobsRuns,
		hostAPIAutomationTriggersPath:                                  handler.handleAutomationTriggers,
		hostAPIAutomationTriggersGetPath:                               handler.handleAutomationTriggersGet,
		hostAPIAutomationTriggersCreatePath:                            handler.handleAutomationTriggersCreate,
		hostAPIAutomationTriggersUpdatePath:                            handler.handleAutomationTriggersUpdate,
		hostAPIAutomationTriggersDeletePath:                            handler.handleAutomationTriggersDelete,
		hostAPIAutomationTriggersRunsPath:                              handler.handleAutomationTriggersRuns,
		hostAPIAutomationTriggersFirePath:                              handler.handleAutomationTriggersFire,
		hostAPIAutomationRunsPath:                                      handler.handleAutomationRuns,
		string(extensioncontract.HostAPIMethodTasks):                   handler.handleTasks,
		string(extensioncontract.HostAPIMethodTasksGet):                handler.handleTasksGet,
		string(extensioncontract.HostAPIMethodTasksTimeline):           handler.handleTasksTimeline,
		string(extensioncontract.HostAPIMethodTasksTree):               handler.handleTasksTree,
		string(extensioncontract.HostAPIMethodTasksDashboard):          handler.handleTasksDashboard,
		string(extensioncontract.HostAPIMethodTasksInbox):              handler.handleTasksInbox,
		string(extensioncontract.HostAPIMethodTasksCreate):             handler.handleTasksCreate,
		string(extensioncontract.HostAPIMethodTasksUpdate):             handler.handleTasksUpdate,
		string(extensioncontract.HostAPIMethodTasksCancel):             handler.handleTasksCancel,
		string(extensioncontract.HostAPIMethodTasksRuns):               handler.handleTasksRuns,
		string(extensioncontract.HostAPIMethodTasksRunsGet):            handler.handleTasksRunsGet,
		string(extensioncontract.HostAPIMethodTasksRunsEnqueue):        handler.handleTasksRunsEnqueue,
		string(extensioncontract.HostAPIMethodTasksRunsStart):          handler.handleTasksRunsStart,
		string(extensioncontract.HostAPIMethodTasksRunsAttachSession):  handler.handleTasksRunsAttachSession,
		string(extensioncontract.HostAPIMethodTasksRunsComplete):       handler.handleTasksRunsComplete,
		string(extensioncontract.HostAPIMethodTasksRunsFail):           handler.handleTasksRunsFail,
		string(extensioncontract.HostAPIMethodTasksRunsCancel):         handler.handleTasksRunsCancel,
		hostAPIResourcesListPath:                                       handler.handleResourcesList,
		hostAPIResourcesGetPath:                                        handler.handleResourcesGet,
		hostAPIResourcesSnapshotPath:                                   handler.handleResourcesSnapshot,
		hostAPIBridgesInstancesListPath:                                handler.handleBridgesInstancesList,
		hostAPIBridgesInstancesGetPath:                                 handler.handleBridgesInstancesGet,
		hostAPIBridgesInstancesReportStatePath:                         handler.handleBridgesInstancesReportState,
		hostAPIBridgesMessagesIngestPath:                               handler.handleBridgesMessagesIngest,
		hostAPISandboxExecPath:                                         handler.handleSandboxExec,
		hostAPISandboxInfoPath:                                         handler.handleSandboxInfo,
		hostAPISandboxListPath:                                         handler.handleSandboxList,
		hostAPIMemoryForgetPath:                                        handler.handleMemoryForget,
		hostAPIMemoryRecallPath:                                        handler.handleMemoryRecall,
		hostAPIMemoryStorePath:                                         handler.handleMemoryStore,
		hostAPIListLogsPath:                                            handler.handleListLogs,
		hostAPIObserveHealthPath:                                       handler.handleObserveHealth,
		string(extensioncontract.HostAPIMethodModelsList):              handler.handleModelsList,
		string(extensioncontract.HostAPIMethodModelsRefresh):           handler.handleModelsRefresh,
		string(extensioncontract.HostAPIMethodModelsStatus):            handler.handleModelsStatus,
		string(extensioncontract.HostAPIMethodAgentsSoulGet):           handler.handleAgentsSoulGet,
		string(extensioncontract.HostAPIMethodAgentsSoulValidate):      handler.handleAgentsSoulValidate,
		string(extensioncontract.HostAPIMethodAgentsSoulPut):           handler.handleAgentsSoulPut,
		string(extensioncontract.HostAPIMethodAgentsSoulDelete):        handler.handleAgentsSoulDelete,
		string(extensioncontract.HostAPIMethodAgentsSoulHistory):       handler.handleAgentsSoulHistory,
		string(extensioncontract.HostAPIMethodAgentsSoulRollback):      handler.handleAgentsSoulRollback,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatGet):      handler.handleAgentsHeartbeatGet,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatValidate): handler.handleAgentsHeartbeatValidate,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatPut):      handler.handleAgentsHeartbeatPut,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatDelete):   handler.handleAgentsHeartbeatDelete,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatHistory):  handler.handleAgentsHeartbeatHistory,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatRollback): handler.handleAgentsHeartbeatRollback,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatStatus):   handler.handleAgentsHeartbeatStatus,
		string(extensioncontract.HostAPIMethodAgentsHeartbeatWake):     handler.handleAgentsHeartbeatWake,
		hostAPISkillsListPath:                                          handler.handleSkillsList,
	}
	registerHostAPISessionMethodHandlers(handler, handlers)
	registerHostAPINetworkMethodHandlers(handler, handlers)
	registerHostAPIClarifyMethodHandler(handler, handlers)
	return handlers
}

func registerHostAPISessionMethodHandlers(
	handler *HostAPIHandler,
	handlers map[string]hostAPIMethodFunc,
) {
	handlers[hostAPISessionsCreatePath] = handler.handleSessionsCreate
	handlers[hostAPISessionsEventsPath] = handler.handleSessionsEvents
	handlers[string(extensioncontract.HostAPIMethodSessionsSoulRefresh)] = handler.handleSessionsSoulRefresh
	handlers[string(extensioncontract.HostAPIMethodSessionsHealthGet)] = handler.handleSessionsHealthGet
	handlers[hostAPISessionsListPath] = handler.handleSessionsList
	handlers[hostAPISessionsPromptPath] = handler.handleSessionsPrompt
	handlers[hostAPISessionsInputsListPath] = handler.handleSessionsInputsList
	handlers[hostAPISessionsInputsReplacePath] = handler.handleSessionsInputsReplace
	handlers[hostAPISessionsInputsCancelPath] = handler.handleSessionsInputsCancel
	handlers[hostAPISessionsInputsPromotePath] = handler.handleSessionsInputsPromote
	handlers[hostAPISessionsStatusPath] = handler.handleSessionsStatus
	handlers[string(extensioncontract.HostAPIMethodSessionsStatusGet)] = handler.handleSessionsStatusGet
	handlers[hostAPISessionsStopPath] = handler.handleSessionsStop
}
