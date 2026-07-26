package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/store"
)

// AgentCreateScope identifies where a newly authored AGENT.md definition is stored.
type AgentCreateScope string

const (
	// AgentCreateScopeWorkspace writes the agent under one workspace's .agh directory.
	AgentCreateScopeWorkspace AgentCreateScope = "workspace"
	// AgentCreateScopeGlobal writes the agent under AGH_HOME.
	AgentCreateScopeGlobal AgentCreateScope = "global"
)

// CreateAgentRequest is the shared agent definition authoring request payload.
type CreateAgentRequest struct {
	Scope     AgentCreateScope   `json:"scope"`
	Workspace string             `json:"workspace,omitempty"`
	Agent     CreateAgentPayload `json:"agent"`
}

// CreateAgentPayload captures the simple AGENT.md fields supported by v1 authoring.
type CreateAgentPayload struct {
	Name            string                   `json:"name"`
	Provider        string                   `json:"provider,omitempty"`
	Command         string                   `json:"command,omitempty"`
	Model           string                   `json:"model,omitempty"`
	ReasoningEffort ReasoningEffort          `json:"reasoning_effort,omitempty"`
	Tools           []string                 `json:"tools,omitempty"`
	Toolsets        []string                 `json:"toolsets,omitempty"`
	DenyTools       []string                 `json:"deny_tools,omitempty"`
	Permissions     SettingsPermissionMode   `json:"permissions,omitempty"`
	CategoryPath    []string                 `json:"category_path,omitempty"`
	Skills          *CreateAgentSkillsConfig `json:"skills,omitempty"`
	Prompt          string                   `json:"prompt"`
}

// CreateAgentSkillsConfig captures agent-local skill policy stored in AGENT.md.
type CreateAgentSkillsConfig struct {
	Disabled []string `json:"disabled,omitempty"`
}

// AgentDiagnosticPayload reports one malformed agent definition encountered during discovery.
type AgentDiagnosticPayload struct {
	Path      string `json:"path"`
	ErrorKind string `json:"error_kind"`
	Message   string `json:"message"`
}

// AgentMCPServerJSON is the shared MCP server response payload.
type AgentMCPServerJSON struct {
	Name      string                            `json:"name"`
	Transport string                            `json:"transport,omitempty"`
	Command   string                            `json:"command,omitempty"`
	Args      []string                          `json:"args,omitempty"`
	Env       map[string]string                 `json:"env,omitempty"`
	SecretEnv map[string]string                 `json:"secret_env,omitempty"`
	URL       string                            `json:"url,omitempty"`
	Auth      *SettingsMCPAuthConfigViewPayload `json:"auth,omitempty"`
}

// AgentEventPayload is the shared raw agent-event streaming payload.
type AgentEventPayload struct {
	Type              string                       `json:"type"`
	SessionID         string                       `json:"session_id,omitempty"`
	TurnID            string                       `json:"turn_id,omitempty"`
	ClientMessageID   string                       `json:"client_message_id,omitempty"`
	RequestID         string                       `json:"request_id,omitempty"`
	Timestamp         time.Time                    `json:"timestamp"`
	Text              string                       `json:"text,omitempty"`
	Title             string                       `json:"title,omitempty"`
	ToolCallID        string                       `json:"tool_call_id,omitempty"`
	StopReason        string                       `json:"stop_reason,omitempty"`
	PromptStopReason  ACPPromptStopReason          `json:"prompt_stop_reason,omitempty"`
	AvailableCommands []ACPAvailableCommandPayload `json:"available_commands,omitempty"`
	Action            string                       `json:"action,omitempty"`
	Resource          string                       `json:"resource,omitempty"`
	Decision          string                       `json:"decision,omitempty"`
	Error             string                       `json:"error,omitempty"`
	Failure           *SessionFailurePayload       `json:"failure,omitempty"`
	Goal              *GoalPromptMeta              `json:"goal,omitempty"`
	Usage             *TokenUsagePayload           `json:"usage,omitempty"`
	Runtime           *RuntimeActivityPayload      `json:"runtime,omitempty"`
	Raw               json.RawMessage              `json:"raw,omitempty"`
}

// TokenUsagePayload is the shared token-usage response payload.
type TokenUsagePayload struct {
	TurnID           string    `json:"turn_id,omitempty"`
	InputTokens      *int64    `json:"input_tokens,omitempty"`
	OutputTokens     *int64    `json:"output_tokens,omitempty"`
	TotalTokens      *int64    `json:"total_tokens,omitempty"`
	ThoughtTokens    *int64    `json:"thought_tokens,omitempty"`
	CacheReadTokens  *int64    `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens *int64    `json:"cache_write_tokens,omitempty"`
	ContextUsed      *int64    `json:"context_used,omitempty"`
	ContextSize      *int64    `json:"context_size,omitempty"`
	CostAmount       *float64  `json:"cost_amount,omitempty"`
	CostCurrency     *string   `json:"cost_currency,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

// LogEventPayload is the shared runtime log response payload.
type LogEventPayload struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	Type        string          `json:"type"`
	AgentName   string          `json:"agent_name"`
	Provider    string          `json:"provider,omitempty"`
	Component   string          `json:"component,omitempty"`
	Outcome     string          `json:"outcome,omitempty"`
	Content     json.RawMessage `json:"content,omitempty"`
	store.EventCorrelation
	ParentSessionID string    `json:"parent_session_id,omitempty"`
	RootSessionID   string    `json:"root_session_id,omitempty"`
	SpawnDepth      int       `json:"spawn_depth"`
	Summary         string    `json:"summary,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

// ObserveHealthPayload is the shared observability health response payload.
type ObserveHealthPayload struct {
	Status             string                          `json:"status"`
	UptimeSeconds      int64                           `json:"uptime_seconds"`
	ActiveSessions     int                             `json:"active_sessions"`
	ActiveAgents       int                             `json:"active_agents"`
	GlobalDBSizeBytes  int64                           `json:"global_db_size_bytes"`
	SessionDBSizeBytes int64                           `json:"session_db_size_bytes"`
	Persistence        ObservePersistenceHealthPayload `json:"persistence"`
	Retention          ObserveRetentionHealthPayload   `json:"retention"`
	Failures           ObserveFailureHealthPayload     `json:"failures"`
	AgentProbes        []AgentProbeHealthPayload       `json:"agent_probes,omitempty"`
	Bridges            BridgeAggregateHealthPayload    `json:"bridges"`
	Activities         []SessionActivityHealthPayload  `json:"activities,omitempty"`
	Version            string                          `json:"version"`
}

// ObserveFailureHealthPayload summarizes persisted lifecycle failures.
type ObserveFailureHealthPayload struct {
	Status string                        `json:"status"`
	Total  int                           `json:"total"`
	ByKind map[store.FailureKind]int     `json:"by_kind,omitempty"`
	Recent []SessionFailureHealthPayload `json:"recent,omitempty"`
}

// SessionFailureHealthPayload exposes one compact lifecycle failure health row.
type SessionFailureHealthPayload struct {
	SessionID       string            `json:"session_id"`
	AgentName       string            `json:"agent_name,omitempty"`
	Provider        string            `json:"provider,omitempty"`
	WorkspaceID     string            `json:"workspace_id,omitempty"`
	State           string            `json:"state,omitempty"`
	FailureKind     store.FailureKind `json:"failure_kind"`
	Summary         string            `json:"summary,omitempty"`
	CrashBundlePath string            `json:"crash_bundle_path,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// AgentProbeHealthPayload exposes one downstream ACP command probe result.
type AgentProbeHealthPayload struct {
	AgentName  string    `json:"agent_name,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	Command    string    `json:"command,omitempty"`
	Executable string    `json:"executable,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
	DurationMS int64     `json:"duration_ms"`
}

// ObservePersistenceHealthPayload captures store health fields shared by
// lifecycle, memory, and operator diagnostics.
type ObservePersistenceHealthPayload struct {
	Status             string `json:"status"`
	GlobalDBSizeBytes  int64  `json:"global_db_size_bytes"`
	SessionDBSizeBytes int64  `json:"session_db_size_bytes"`
}

// ObserveRetentionHealthPayload captures the observable state of configured
// retention sweeps.
type ObserveRetentionHealthPayload struct {
	Enabled                  bool       `json:"enabled"`
	RetentionDays            int        `json:"retention_days"`
	SweepIntervalSeconds     int64      `json:"sweep_interval_seconds"`
	LastSweepStatus          string     `json:"last_sweep_status"`
	LastSweepAt              *time.Time `json:"last_sweep_at,omitempty"`
	LastCutoffAt             *time.Time `json:"last_cutoff_at,omitempty"`
	LastSweepError           string     `json:"last_sweep_error,omitempty"`
	DeletedEventSummaries    int64      `json:"deleted_event_summaries"`
	DeletedTokenStats        int64      `json:"deleted_token_stats"`
	DeletedTokenUsageDaily   int64      `json:"deleted_token_usage_daily"`
	DeletedPermissionLogRows int64      `json:"deleted_permission_log_rows"`
}

// SessionActivityHealthPayload exposes active runtime supervision state in the
// observability health response.
type SessionActivityHealthPayload struct {
	SessionID          string     `json:"session_id"`
	TurnID             string     `json:"turn_id,omitempty"`
	TurnSource         string     `json:"turn_source,omitempty"`
	TurnStartedAt      *time.Time `json:"turn_started_at,omitempty"`
	DeadlineAt         *time.Time `json:"deadline_at,omitempty"`
	LastActivityAt     *time.Time `json:"last_activity_at,omitempty"`
	LastActivityKind   string     `json:"last_activity_kind,omitempty"`
	LastActivityDetail string     `json:"last_activity_detail,omitempty"`
	CurrentTool        string     `json:"current_tool,omitempty"`
	ToolCallID         string     `json:"tool_call_id,omitempty"`
	LastProgressAt     *time.Time `json:"last_progress_at,omitempty"`
	IterationCurrent   int        `json:"iteration_current"`
	IterationMax       int        `json:"iteration_max"`
	IdleSeconds        int64      `json:"idle_seconds"`
	ElapsedSeconds     int64      `json:"elapsed_seconds"`
	ElapsedMS          int64      `json:"elapsed_ms"`
	Status             string     `json:"status"`
	StallState         string     `json:"stall_state,omitempty"`
	StallReason        string     `json:"stall_reason,omitempty"`
}
