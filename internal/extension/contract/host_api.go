package contract

import (
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/network/participation"
)

const (
	hostAPIAgentSoulMutationResponseValue = "AgentSoulMutationResponse"
	hostAPIAgentSoulPayloadValue          = "AgentSoulPayload"
	hostAPIAutomationTargetParamsValue    = "AutomationTargetParams"
	hostAPIBridgeInstanceValue            = "BridgeInstance"
	hostAPIEmptyResultValue               = "EmptyResult"
	hostAPIHeartbeatMutationResponseValue = "HeartbeatMutationResponse"
	hostAPIHeartbeatPolicyPayloadValue    = "HeartbeatPolicyPayload"
	hostAPIJobValue                       = "Job"
	hostAPIResourceRecordValue            = "ResourceRecord"
	hostAPIRunValue                       = "Run"
	hostAPITaskValue                      = "Task"
	hostAPITaskRunValue                   = "TaskRun"
	hostAPITriggerValue                   = "Trigger"
)

// HostAPIMethod identifies one extension -> AGH Host API request.
type HostAPIMethod = extensionprotocol.HostAPIMethod

const (
	HostAPIMethodSessionsList                = extensionprotocol.HostAPIMethodSessionsList
	HostAPIMethodSessionsCreate              = extensionprotocol.HostAPIMethodSessionsCreate
	HostAPIMethodSessionsPrompt              = extensionprotocol.HostAPIMethodSessionsPrompt
	HostAPIMethodSessionsStop                = extensionprotocol.HostAPIMethodSessionsStop
	HostAPIMethodSessionsStatus              = extensionprotocol.HostAPIMethodSessionsStatus
	HostAPIMethodSessionsEvents              = extensionprotocol.HostAPIMethodSessionsEvents
	HostAPIMethodSessionsSoulRefresh         = extensionprotocol.HostAPIMethodSessionsSoulRefresh
	HostAPIMethodSessionsHealthGet           = extensionprotocol.HostAPIMethodSessionsHealthGet
	HostAPIMethodSessionsStatusGet           = extensionprotocol.HostAPIMethodSessionsStatusGet
	HostAPIMethodSandboxList                 = extensionprotocol.HostAPIMethodSandboxList
	HostAPIMethodSandboxInfo                 = extensionprotocol.HostAPIMethodSandboxInfo
	HostAPIMethodSandboxExec                 = extensionprotocol.HostAPIMethodSandboxExec
	HostAPIMethodMemoryRecall                = extensionprotocol.HostAPIMethodMemoryRecall
	HostAPIMethodMemoryStore                 = extensionprotocol.HostAPIMethodMemoryStore
	HostAPIMethodMemoryForget                = extensionprotocol.HostAPIMethodMemoryForget
	HostAPIMethodObserveHealth               = extensionprotocol.HostAPIMethodObserveHealth
	HostAPIMethodListLogs                    = extensionprotocol.HostAPIMethodListLogs
	HostAPIMethodSkillsList                  = extensionprotocol.HostAPIMethodSkillsList
	HostAPIMethodModelsList                  = extensionprotocol.HostAPIMethodModelsList
	HostAPIMethodModelsRefresh               = extensionprotocol.HostAPIMethodModelsRefresh
	HostAPIMethodModelsStatus                = extensionprotocol.HostAPIMethodModelsStatus
	HostAPIMethodAgentsSoulGet               = extensionprotocol.HostAPIMethodAgentsSoulGet
	HostAPIMethodAgentsSoulValidate          = extensionprotocol.HostAPIMethodAgentsSoulValidate
	HostAPIMethodAgentsSoulPut               = extensionprotocol.HostAPIMethodAgentsSoulPut
	HostAPIMethodAgentsSoulDelete            = extensionprotocol.HostAPIMethodAgentsSoulDelete
	HostAPIMethodAgentsSoulHistory           = extensionprotocol.HostAPIMethodAgentsSoulHistory
	HostAPIMethodAgentsSoulRollback          = extensionprotocol.HostAPIMethodAgentsSoulRollback
	HostAPIMethodAgentsHeartbeatGet          = extensionprotocol.HostAPIMethodAgentsHeartbeatGet
	HostAPIMethodAgentsHeartbeatValidate     = extensionprotocol.HostAPIMethodAgentsHeartbeatValidate
	HostAPIMethodAgentsHeartbeatPut          = extensionprotocol.HostAPIMethodAgentsHeartbeatPut
	HostAPIMethodAgentsHeartbeatDelete       = extensionprotocol.HostAPIMethodAgentsHeartbeatDelete
	HostAPIMethodAgentsHeartbeatHistory      = extensionprotocol.HostAPIMethodAgentsHeartbeatHistory
	HostAPIMethodAgentsHeartbeatRollback     = extensionprotocol.HostAPIMethodAgentsHeartbeatRollback
	HostAPIMethodAgentsHeartbeatStatus       = extensionprotocol.HostAPIMethodAgentsHeartbeatStatus
	HostAPIMethodAgentsHeartbeatWake         = extensionprotocol.HostAPIMethodAgentsHeartbeatWake
	HostAPIMethodAutomationJobs              = extensionprotocol.HostAPIMethodAutomationJobs
	HostAPIMethodAutomationJobsGet           = extensionprotocol.HostAPIMethodAutomationJobsGet
	HostAPIMethodAutomationJobsCreate        = extensionprotocol.HostAPIMethodAutomationJobsCreate
	HostAPIMethodAutomationJobsUpdate        = extensionprotocol.HostAPIMethodAutomationJobsUpdate
	HostAPIMethodAutomationJobsDelete        = extensionprotocol.HostAPIMethodAutomationJobsDelete
	HostAPIMethodAutomationJobsTrigger       = extensionprotocol.HostAPIMethodAutomationJobsTrigger
	HostAPIMethodAutomationJobsRuns          = extensionprotocol.HostAPIMethodAutomationJobsRuns
	HostAPIMethodAutomationTriggers          = extensionprotocol.HostAPIMethodAutomationTriggers
	HostAPIMethodAutomationTriggersGet       = extensionprotocol.HostAPIMethodAutomationTriggersGet
	HostAPIMethodAutomationTriggersCreate    = extensionprotocol.HostAPIMethodAutomationTriggersCreate
	HostAPIMethodAutomationTriggersUpdate    = extensionprotocol.HostAPIMethodAutomationTriggersUpdate
	HostAPIMethodAutomationTriggersDelete    = extensionprotocol.HostAPIMethodAutomationTriggersDelete
	HostAPIMethodAutomationTriggersRuns      = extensionprotocol.HostAPIMethodAutomationTriggersRuns
	HostAPIMethodAutomationTriggersFire      = extensionprotocol.HostAPIMethodAutomationTriggersFire
	HostAPIMethodAutomationRuns              = extensionprotocol.HostAPIMethodAutomationRuns
	HostAPIMethodTasks                       = extensionprotocol.HostAPIMethodTasks
	HostAPIMethodTasksGet                    = extensionprotocol.HostAPIMethodTasksGet
	HostAPIMethodTasksTimeline               = extensionprotocol.HostAPIMethodTasksTimeline
	HostAPIMethodTasksTree                   = extensionprotocol.HostAPIMethodTasksTree
	HostAPIMethodTasksDashboard              = extensionprotocol.HostAPIMethodTasksDashboard
	HostAPIMethodTasksInbox                  = extensionprotocol.HostAPIMethodTasksInbox
	HostAPIMethodTasksCreate                 = extensionprotocol.HostAPIMethodTasksCreate
	HostAPIMethodTasksUpdate                 = extensionprotocol.HostAPIMethodTasksUpdate
	HostAPIMethodTasksCancel                 = extensionprotocol.HostAPIMethodTasksCancel
	HostAPIMethodTasksRuns                   = extensionprotocol.HostAPIMethodTasksRuns
	HostAPIMethodTasksRunsGet                = extensionprotocol.HostAPIMethodTasksRunsGet
	HostAPIMethodTasksRunsEnqueue            = extensionprotocol.HostAPIMethodTasksRunsEnqueue
	HostAPIMethodTasksRunsStart              = extensionprotocol.HostAPIMethodTasksRunsStart
	HostAPIMethodTasksRunsAttachSession      = extensionprotocol.HostAPIMethodTasksRunsAttachSession
	HostAPIMethodTasksRunsComplete           = extensionprotocol.HostAPIMethodTasksRunsComplete
	HostAPIMethodTasksRunsFail               = extensionprotocol.HostAPIMethodTasksRunsFail
	HostAPIMethodTasksRunsCancel             = extensionprotocol.HostAPIMethodTasksRunsCancel
	HostAPIMethodNetworkStatus               = extensionprotocol.HostAPIMethodNetworkStatus
	HostAPIMethodNetworkUsage                = extensionprotocol.HostAPIMethodNetworkUsage
	HostAPIMethodNetworkChannels             = extensionprotocol.HostAPIMethodNetworkChannels
	HostAPIMethodNetworkPeers                = extensionprotocol.HostAPIMethodNetworkPeers
	HostAPIMethodNetworkThreads              = extensionprotocol.HostAPIMethodNetworkThreads
	HostAPIMethodNetworkThreadGet            = extensionprotocol.HostAPIMethodNetworkThreadGet
	HostAPIMethodNetworkThreadMessages       = extensionprotocol.HostAPIMethodNetworkThreadMessages
	HostAPIMethodNetworkDirects              = extensionprotocol.HostAPIMethodNetworkDirects
	HostAPIMethodNetworkDirectResolve        = extensionprotocol.HostAPIMethodNetworkDirectResolve
	HostAPIMethodNetworkDirectMessages       = extensionprotocol.HostAPIMethodNetworkDirectMessages
	HostAPIMethodNetworkWorkGet              = extensionprotocol.HostAPIMethodNetworkWorkGet
	HostAPIMethodNetworkSend                 = extensionprotocol.HostAPIMethodNetworkSend
	HostAPIMethodResourcesList               = extensionprotocol.HostAPIMethodResourcesList
	HostAPIMethodResourcesGet                = extensionprotocol.HostAPIMethodResourcesGet
	HostAPIMethodResourcesSnapshot           = extensionprotocol.HostAPIMethodResourcesSnapshot
	HostAPIMethodBridgesInstancesList        = extensionprotocol.HostAPIMethodBridgesInstancesList
	HostAPIMethodBridgesMessagesIngest       = extensionprotocol.HostAPIMethodBridgesMessagesIngest
	HostAPIMethodBridgesInstancesGet         = extensionprotocol.HostAPIMethodBridgesInstancesGet
	HostAPIMethodBridgesInstancesReportState = extensionprotocol.HostAPIMethodBridgesInstancesReportState
)

// NamedType links a generated TypeScript export name to a Go type.
type NamedType struct {
	Name  string
	Value any
}

// HostAPIMethodSpec describes one Host API request/response contract.
type HostAPIMethodSpec struct {
	Method         HostAPIMethod
	Params         NamedType
	Result         NamedType
	OptionalParams bool
}

// EmptyResult is the empty JSON-RPC result for methods without payloads.
type EmptyResult struct{}

// SessionsListParams filters visible sessions.
type SessionsListParams struct {
	Workspace string `json:"workspace,omitempty"`
}

// SessionsCreateParams starts a new session.
type SessionsCreateParams struct {
	Agent                string                      `json:"agent"`
	Prompt               string                      `json:"prompt,omitempty"`
	Provider             string                      `json:"provider,omitempty"`
	Model                string                      `json:"model,omitempty"`
	ReasoningEffort      apicontract.ReasoningEffort `json:"reasoning_effort,omitempty"`
	Workspace            string                      `json:"workspace,omitempty"`
	NetworkParticipation *participation.Request      `json:"network_participation,omitempty"`
}

// SessionsPromptParams submits one prompt to an existing session.
type SessionsPromptParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	Message     string `json:"message"`
}

// SessionTargetParams identifies an existing session.
type SessionTargetParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

// SessionEventsParams filters persisted session events.
type SessionEventsParams struct {
	WorkspaceID string    `json:"workspace_id"`
	SessionID   string    `json:"session_id"`
	Type        string    `json:"type,omitempty"`
	AgentName   string    `json:"agent_name,omitempty"`
	TurnID      string    `json:"turn_id,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Offset      int64     `json:"offset,omitempty"`
	Since       time.Time `json:"since,omitzero"`
}

// SessionSoulRefreshParams refreshes one session's Soul snapshot through managed CAS.
type SessionSoulRefreshParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	apicontract.SessionSoulRefreshRequest
}

// SessionHealthGetParams identifies one session health row.
type SessionHealthGetParams = SessionTargetParams

// SessionStatusGetParams identifies one authored-context session status row.
type SessionStatusGetParams = SessionTargetParams

// SandboxListParams filters active sandboxes.
type SandboxListParams struct {
	Workspace string `json:"workspace,omitempty"`
}

// SandboxInfoParams identifies one session sandbox.
type SandboxInfoParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

// SandboxExecParams executes one command inside a session sandbox.
type SandboxExecParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	Command     string `json:"command"`
	Timeout     int    `json:"timeout,omitempty"`
}

// MemoryStoreParams persists one memory document.
type MemoryStoreParams struct {
	Key       string            `json:"key"`
	Content   string            `json:"content"`
	Scope     memcontract.Scope `json:"scope,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
}

// MemoryRecallParams queries stored memory documents.
type MemoryRecallParams struct {
	Query     string            `json:"query"`
	Limit     int               `json:"limit,omitempty"`
	Scope     memcontract.Scope `json:"scope,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
}

// MemoryForgetParams removes one stored memory document.
type MemoryForgetParams struct {
	Key       string            `json:"key"`
	Scope     memcontract.Scope `json:"scope,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
}

// ListLogsParams filters workspace runtime logs.
type ListLogsParams struct {
	WorkspaceID   string    `json:"workspace_id"`
	SessionID     string    `json:"session_id,omitempty"`
	AgentName     string    `json:"agent_name,omitempty"`
	Type          string    `json:"type,omitempty"`
	RunID         string    `json:"run,omitempty"`
	ActorKind     string    `json:"actor_kind,omitempty"`
	ActorID       string    `json:"actor_id,omitempty"`
	Provider      string    `json:"provider,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
	Component     string    `json:"component,omitempty"`
	ErrorOnly     bool      `json:"error_only,omitempty"`
	AfterSequence int64     `json:"after_seq,omitempty"`
	Since         time.Time `json:"since,omitzero"`
	Limit         int       `json:"limit,omitempty"`
}

// SkillsListParams filters skills by workspace scope.
type SkillsListParams struct {
	Workspace string `json:"workspace,omitempty"`
	ForAgent  string `json:"for_agent,omitempty"`
}

// ModelsListParams filters daemon-owned model catalog projections.
type ModelsListParams struct {
	ProviderID   string `json:"provider_id,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	Refresh      bool   `json:"refresh,omitempty"`
	IncludeStale bool   `json:"include_stale,omitempty"`
}

// ModelsRefreshParams requests a daemon-owned model catalog refresh.
type ModelsRefreshParams struct {
	ProviderID string `json:"provider_id,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	Force      bool   `json:"force,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

// ModelsStatusParams filters daemon-owned model catalog source status rows.
type ModelsStatusParams struct {
	ProviderID string `json:"provider_id,omitempty"`
}
