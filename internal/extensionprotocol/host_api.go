package protocol

// HostAPIMethod identifies one extension -> Compozy Host API request.
type HostAPIMethod string

const (
	HostAPIMethodSessionsList                HostAPIMethod = "sessions/list"
	HostAPIMethodSessionsCreate              HostAPIMethod = "sessions/create"
	HostAPIMethodSessionsPrompt              HostAPIMethod = "sessions/prompt"
	HostAPIMethodSessionsInputsList          HostAPIMethod = "sessions/inputs/list"
	HostAPIMethodSessionsInputsReplace       HostAPIMethod = "sessions/inputs/replace"
	HostAPIMethodSessionsInputsCancel        HostAPIMethod = "sessions/inputs/cancel"
	HostAPIMethodSessionsInputsPromote       HostAPIMethod = "sessions/inputs/promote"
	HostAPIMethodSessionsStop                HostAPIMethod = "sessions/stop"
	HostAPIMethodSessionsStatus              HostAPIMethod = "sessions/status"
	HostAPIMethodSessionsEvents              HostAPIMethod = "sessions/events"
	HostAPIMethodSessionsSoulRefresh         HostAPIMethod = "sessions/soul/refresh"
	HostAPIMethodSessionsHealthGet           HostAPIMethod = "sessions/health/get"
	HostAPIMethodSessionsStatusGet           HostAPIMethod = "sessions/status/get"
	HostAPIMethodSandboxList                 HostAPIMethod = "sandbox/list"
	HostAPIMethodSandboxInfo                 HostAPIMethod = "sandbox/info"
	HostAPIMethodSandboxExec                 HostAPIMethod = "sandbox/exec"
	HostAPIMethodMemoryRecall                HostAPIMethod = "memory/recall"
	HostAPIMethodMemoryStore                 HostAPIMethod = "memory/store"
	HostAPIMethodMemoryForget                HostAPIMethod = "memory/forget"
	HostAPIMethodObserveHealth               HostAPIMethod = "observe/health"
	HostAPIMethodListLogs                    HostAPIMethod = "logs/list"
	HostAPIMethodSkillsList                  HostAPIMethod = "skills/list"
	HostAPIMethodModelsList                  HostAPIMethod = "models/list"
	HostAPIMethodModelsRefresh               HostAPIMethod = "models/refresh"
	HostAPIMethodModelsStatus                HostAPIMethod = "models/status"
	HostAPIMethodAgentsSoulGet               HostAPIMethod = "agents/soul/get"
	HostAPIMethodAgentsSoulValidate          HostAPIMethod = "agents/soul/validate"
	HostAPIMethodAgentsSoulPut               HostAPIMethod = "agents/soul/put"
	HostAPIMethodAgentsSoulDelete            HostAPIMethod = "agents/soul/delete"
	HostAPIMethodAgentsSoulHistory           HostAPIMethod = "agents/soul/history"
	HostAPIMethodAgentsSoulRollback          HostAPIMethod = "agents/soul/rollback"
	HostAPIMethodAgentsHeartbeatGet          HostAPIMethod = "agents/heartbeat/get"
	HostAPIMethodAgentsHeartbeatValidate     HostAPIMethod = "agents/heartbeat/validate"
	HostAPIMethodAgentsHeartbeatPut          HostAPIMethod = "agents/heartbeat/put"
	HostAPIMethodAgentsHeartbeatDelete       HostAPIMethod = "agents/heartbeat/delete"
	HostAPIMethodAgentsHeartbeatHistory      HostAPIMethod = "agents/heartbeat/history"
	HostAPIMethodAgentsHeartbeatRollback     HostAPIMethod = "agents/heartbeat/rollback"
	HostAPIMethodAgentsHeartbeatStatus       HostAPIMethod = "agents/heartbeat/status"
	HostAPIMethodAgentsHeartbeatWake         HostAPIMethod = "agents/heartbeat/wake"
	HostAPIMethodAutomationJobs              HostAPIMethod = "automation/jobs"
	HostAPIMethodAutomationJobsGet           HostAPIMethod = "automation/jobs/get"
	HostAPIMethodAutomationJobsCreate        HostAPIMethod = "automation/jobs/create"
	HostAPIMethodAutomationJobsUpdate        HostAPIMethod = "automation/jobs/update"
	HostAPIMethodAutomationJobsDelete        HostAPIMethod = "automation/jobs/delete"
	HostAPIMethodAutomationJobsTrigger       HostAPIMethod = "automation/jobs/trigger"
	HostAPIMethodAutomationJobsRuns          HostAPIMethod = "automation/jobs/runs"
	HostAPIMethodAutomationTriggers          HostAPIMethod = "automation/triggers"
	HostAPIMethodAutomationTriggersGet       HostAPIMethod = "automation/triggers/get"
	HostAPIMethodAutomationTriggersCreate    HostAPIMethod = "automation/triggers/create"
	HostAPIMethodAutomationTriggersUpdate    HostAPIMethod = "automation/triggers/update"
	HostAPIMethodAutomationTriggersDelete    HostAPIMethod = "automation/triggers/delete"
	HostAPIMethodAutomationTriggersRuns      HostAPIMethod = "automation/triggers/runs"
	HostAPIMethodAutomationTriggersFire      HostAPIMethod = "automation/triggers/fire"
	HostAPIMethodAutomationRuns              HostAPIMethod = "automation/runs"
	HostAPIMethodTasks                       HostAPIMethod = "tasks"
	HostAPIMethodTasksGet                    HostAPIMethod = "tasks/get"
	HostAPIMethodTasksTimeline               HostAPIMethod = "tasks/timeline"
	HostAPIMethodTasksTree                   HostAPIMethod = "tasks/tree"
	HostAPIMethodTasksDashboard              HostAPIMethod = "tasks/dashboard"
	HostAPIMethodTasksInbox                  HostAPIMethod = "tasks/inbox"
	HostAPIMethodTasksCreate                 HostAPIMethod = "tasks/create"
	HostAPIMethodTasksUpdate                 HostAPIMethod = "tasks/update"
	HostAPIMethodTasksCancel                 HostAPIMethod = "tasks/cancel"
	HostAPIMethodTasksRuns                   HostAPIMethod = "tasks/runs"
	HostAPIMethodTasksRunsGet                HostAPIMethod = "tasks/runs/get"
	HostAPIMethodTasksRunsEnqueue            HostAPIMethod = "tasks/runs/enqueue"
	HostAPIMethodTasksRunsStart              HostAPIMethod = "tasks/runs/start"
	HostAPIMethodTasksRunsAttachSession      HostAPIMethod = "tasks/runs/attach_session"
	HostAPIMethodTasksRunsComplete           HostAPIMethod = "tasks/runs/complete"
	HostAPIMethodTasksRunsFail               HostAPIMethod = "tasks/runs/fail"
	HostAPIMethodTasksRunsCancel             HostAPIMethod = "tasks/runs/cancel"
	HostAPIMethodNetworkStatus               HostAPIMethod = "network/status"
	HostAPIMethodNetworkUsage                HostAPIMethod = "network/usage"
	HostAPIMethodNetworkChannels             HostAPIMethod = "network/channels"
	HostAPIMethodNetworkPeers                HostAPIMethod = "network/peers"
	HostAPIMethodNetworkThreads              HostAPIMethod = "network/threads"
	HostAPIMethodNetworkThreadGet            HostAPIMethod = "network/thread/get"
	HostAPIMethodNetworkThreadMessages       HostAPIMethod = "network/thread/messages"
	HostAPIMethodNetworkDirects              HostAPIMethod = "network/directs"
	HostAPIMethodNetworkDirectResolve        HostAPIMethod = "network/direct/resolve"
	HostAPIMethodNetworkDirectMessages       HostAPIMethod = "network/direct/messages"
	HostAPIMethodNetworkWorkGet              HostAPIMethod = "network/work/get"
	HostAPIMethodNetworkSend                 HostAPIMethod = "network/send"
	HostAPIMethodResourcesList               HostAPIMethod = "resources/list"
	HostAPIMethodResourcesGet                HostAPIMethod = "resources/get"
	HostAPIMethodResourcesSnapshot           HostAPIMethod = "resources/snapshot"
	HostAPIMethodBridgesInstancesList        HostAPIMethod = "bridges/instances/list"
	HostAPIMethodBridgesMessagesIngest       HostAPIMethod = "bridges/messages/ingest"
	HostAPIMethodBridgesInstancesGet         HostAPIMethod = "bridges/instances/get"
	HostAPIMethodBridgesInstancesReportState HostAPIMethod = "bridges/instances/report_state"
	HostAPIMethodClarifyAsk                  HostAPIMethod = "clarify/ask"
)

// AllHostAPIMethods returns the canonical Host API method registry in wire order.
func AllHostAPIMethods() []HostAPIMethod {
	methods := preNetworkHostAPIMethods()
	methods = append(methods, networkHostAPIMethods()...)
	methods = append(methods,
		HostAPIMethodResourcesList,
		HostAPIMethodResourcesGet,
		HostAPIMethodResourcesSnapshot,
		HostAPIMethodBridgesInstancesList,
		HostAPIMethodBridgesMessagesIngest,
		HostAPIMethodBridgesInstancesGet,
		HostAPIMethodBridgesInstancesReportState,
		HostAPIMethodClarifyAsk,
	)
	return methods
}

func preNetworkHostAPIMethods() []HostAPIMethod {
	return []HostAPIMethod{
		HostAPIMethodSessionsList,
		HostAPIMethodSessionsCreate,
		HostAPIMethodSessionsPrompt,
		HostAPIMethodSessionsInputsList,
		HostAPIMethodSessionsInputsReplace,
		HostAPIMethodSessionsInputsCancel,
		HostAPIMethodSessionsInputsPromote,
		HostAPIMethodSessionsStop,
		HostAPIMethodSessionsStatus,
		HostAPIMethodSessionsEvents,
		HostAPIMethodSessionsSoulRefresh,
		HostAPIMethodSessionsHealthGet,
		HostAPIMethodSessionsStatusGet,
		HostAPIMethodSandboxList,
		HostAPIMethodSandboxInfo,
		HostAPIMethodSandboxExec,
		HostAPIMethodMemoryRecall,
		HostAPIMethodMemoryStore,
		HostAPIMethodMemoryForget,
		HostAPIMethodObserveHealth,
		HostAPIMethodListLogs,
		HostAPIMethodSkillsList,
		HostAPIMethodModelsList,
		HostAPIMethodModelsRefresh,
		HostAPIMethodModelsStatus,
		HostAPIMethodAgentsSoulGet,
		HostAPIMethodAgentsSoulValidate,
		HostAPIMethodAgentsSoulPut,
		HostAPIMethodAgentsSoulDelete,
		HostAPIMethodAgentsSoulHistory,
		HostAPIMethodAgentsSoulRollback,
		HostAPIMethodAgentsHeartbeatGet,
		HostAPIMethodAgentsHeartbeatValidate,
		HostAPIMethodAgentsHeartbeatPut,
		HostAPIMethodAgentsHeartbeatDelete,
		HostAPIMethodAgentsHeartbeatHistory,
		HostAPIMethodAgentsHeartbeatRollback,
		HostAPIMethodAgentsHeartbeatStatus,
		HostAPIMethodAgentsHeartbeatWake,
		HostAPIMethodAutomationJobs,
		HostAPIMethodAutomationJobsGet,
		HostAPIMethodAutomationJobsCreate,
		HostAPIMethodAutomationJobsUpdate,
		HostAPIMethodAutomationJobsDelete,
		HostAPIMethodAutomationJobsTrigger,
		HostAPIMethodAutomationJobsRuns,
		HostAPIMethodAutomationTriggers,
		HostAPIMethodAutomationTriggersGet,
		HostAPIMethodAutomationTriggersCreate,
		HostAPIMethodAutomationTriggersUpdate,
		HostAPIMethodAutomationTriggersDelete,
		HostAPIMethodAutomationTriggersRuns,
		HostAPIMethodAutomationTriggersFire,
		HostAPIMethodAutomationRuns,
		HostAPIMethodTasks,
		HostAPIMethodTasksGet,
		HostAPIMethodTasksTimeline,
		HostAPIMethodTasksTree,
		HostAPIMethodTasksDashboard,
		HostAPIMethodTasksInbox,
		HostAPIMethodTasksCreate,
		HostAPIMethodTasksUpdate,
		HostAPIMethodTasksCancel,
		HostAPIMethodTasksRuns,
		HostAPIMethodTasksRunsGet,
		HostAPIMethodTasksRunsEnqueue,
		HostAPIMethodTasksRunsStart,
		HostAPIMethodTasksRunsAttachSession,
		HostAPIMethodTasksRunsComplete,
		HostAPIMethodTasksRunsFail,
		HostAPIMethodTasksRunsCancel,
	}
}

func networkHostAPIMethods() []HostAPIMethod {
	return []HostAPIMethod{
		HostAPIMethodNetworkStatus,
		HostAPIMethodNetworkUsage,
		HostAPIMethodNetworkChannels,
		HostAPIMethodNetworkPeers,
		HostAPIMethodNetworkThreads,
		HostAPIMethodNetworkThreadGet,
		HostAPIMethodNetworkThreadMessages,
		HostAPIMethodNetworkDirects,
		HostAPIMethodNetworkDirectResolve,
		HostAPIMethodNetworkDirectMessages,
		HostAPIMethodNetworkWorkGet,
		HostAPIMethodNetworkSend,
	}
}
