package contract

import (
	"time"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// CoordinatorConfigSource identifies where a coordinator config read model came from.
type CoordinatorConfigSource string

const (
	// CoordinatorConfigSourceWorkspace identifies a workspace override.
	CoordinatorConfigSourceWorkspace CoordinatorConfigSource = "workspace"
	// CoordinatorConfigSourceGlobal identifies global config.
	CoordinatorConfigSourceGlobal CoordinatorConfigSource = "global"
	// CoordinatorConfigSourceDefault identifies bundled defaults or agent fallback.
	CoordinatorConfigSourceDefault CoordinatorConfigSource = "default"
)

// AgentIdentityPayload describes the daemon-authenticated caller identity.
type AgentIdentityPayload struct {
	SessionID string `json:"session_id"`
	AgentName string `json:"agent_name"`
	Provider  string `json:"provider"`
	Model     string `json:"model,omitempty"`
}

// AgentWorkspacePayload is the compact workspace context used by agent endpoints.
type AgentWorkspacePayload struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	RootDir string `json:"root_dir,omitempty"`
}

// SessionLineagePayload exposes safe parent/child lineage metadata for spawned sessions.
type SessionLineagePayload struct {
	ParentSessionID  string                       `json:"parent_session_id,omitempty"`
	RootSessionID    string                       `json:"root_session_id,omitempty"`
	SpawnDepth       int                          `json:"spawn_depth"`
	SpawnRole        string                       `json:"spawn_role,omitempty"`
	Kind             store.LineageKind            `json:"kind,omitempty"`
	OriginMessageID  string                       `json:"origin_message_id,omitempty"`
	OriginAgentName  string                       `json:"origin_agent_name,omitempty"`
	TTLExpiresAt     *time.Time                   `json:"ttl_expires_at,omitempty"`
	AutoStopOnParent bool                         `json:"auto_stop_on_parent"`
	NotifyCreator    bool                         `json:"notify_creator"`
	SpawnBudget      SpawnBudgetPayload           `json:"spawn_budget"`
	PermissionPolicy SpawnPermissionPolicyPayload `json:"permission_policy"`
}

// SpawnBudgetPayload is the transport read model for bounded spawn limits.
type SpawnBudgetPayload struct {
	MaxChildren           int   `json:"max_children"`
	MaxDepth              int   `json:"max_depth"`
	TTLSeconds            int64 `json:"ttl_seconds"`
	MaxActivePerWorkspace int   `json:"max_active_per_workspace,omitempty"`
}

// SpawnPermissionPolicyPayload captures concrete permission atoms available to a spawned session.
type SpawnPermissionPolicyPayload struct {
	Tools          []string `json:"tools"`
	Skills         []string `json:"skills"`
	MCPServers     []string `json:"mcp_servers"`
	WorkspacePaths []string `json:"workspace_paths"`
}

// AgentCapabilityPayload describes one caller capability atom.
type AgentCapabilityPayload struct {
	ID      string `json:"id"`
	Summary string `json:"summary,omitempty"`
	Source  string `json:"source,omitempty"`
}

// AgentLimitsPayload reports safe runtime limits relevant to agent decisions.
type AgentLimitsPayload struct {
	MaxChildren         int `json:"max_children"`
	MaxSpawnDepth       int `json:"max_spawn_depth"`
	MaxActiveTaskLeases int `json:"max_active_task_leases"`
	ContextSectionLimit int `json:"context_section_limit"`
}

// CoordinatorConfigPayload is the safe coordinator config read model.
type CoordinatorConfigPayload struct {
	Enabled                       bool                    `json:"enabled"`
	AgentName                     string                  `json:"agent_name"`
	Provider                      string                  `json:"provider,omitempty"`
	Model                         string                  `json:"model,omitempty"`
	DefaultTTLSeconds             int64                   `json:"default_ttl_seconds"`
	MaxChildren                   int                     `json:"max_children"`
	MaxActiveSessionsPerWorkspace int                     `json:"max_active_sessions_per_workspace"`
	Source                        CoordinatorConfigSource `json:"source"`
	WorkspaceID                   string                  `json:"workspace_id,omitempty"`
}

// TaskRunLeaseSummaryPayload is the safe read projection for task-run lease state.
type TaskRunLeaseSummaryPayload struct {
	TaskID         string                 `json:"task_id"`
	RunID          string                 `json:"run_id"`
	Status         taskpkg.RunStatus      `json:"status"`
	SessionID      string                 `json:"session_id,omitempty"`
	ClaimedBy      *taskpkg.ActorIdentity `json:"claimed_by,omitempty"`
	ClaimTokenHash string                 `json:"claim_token_hash,omitempty"`
	LeaseUntil     *time.Time             `json:"lease_until,omitempty"`
	HeartbeatAt    *time.Time             `json:"heartbeat_at,omitempty"`
}

// AgentTaskContextPayload is the bounded active-task section in `/agent/context`.
type AgentTaskContextPayload struct {
	Available bool                        `json:"available"`
	Task      *TaskReferencePayload       `json:"task,omitzero"`
	Lease     *TaskRunLeaseSummaryPayload `json:"lease,omitzero"`
	Bundle    *taskpkg.ContextBundle      `json:"bundle,omitzero"`
}

// AgentContextSectionMetaPayload reports bounding/truncation metadata for context sections.
type AgentContextSectionMetaPayload struct {
	Limit     int  `json:"limit"`
	Returned  int  `json:"returned"`
	Truncated bool `json:"truncated"`
}

// AgentCapabilitySectionPayload is the bounded capability section in `/agent/context`.
type AgentCapabilitySectionPayload struct {
	Section      AgentContextSectionMetaPayload `json:"section"`
	Capabilities []AgentCapabilityPayload       `json:"capabilities"`
}

// AgentContextProvenancePayload describes when and how an agent context was assembled.
type AgentContextProvenancePayload struct {
	GeneratedAt time.Time `json:"generated_at"`
	Source      string    `json:"source"`
}

// AgentMePayload is the compact caller state returned by `/agent/me`.
type AgentMePayload struct {
	Self             AgentIdentityPayload         `json:"self"`
	Workspace        AgentWorkspacePayload        `json:"workspace"`
	Session          AgentSessionPayload          `json:"session"`
	Capabilities     []AgentCapabilityPayload     `json:"capabilities"`
	ActiveTaskLeases []TaskRunLeaseSummaryPayload `json:"active_task_leases"`
	Coordinator      CoordinatorConfigPayload     `json:"coordinator"`
	Limits           AgentLimitsPayload           `json:"limits"`
}

// AgentContextPayload is the stable bounded situation payload returned by `/agent/context`.
type AgentContextPayload struct {
	Soul         AgentSoulSectionPayload       `json:"soul"`
	Self         AgentIdentityPayload          `json:"self"`
	Workspace    AgentWorkspacePayload         `json:"workspace"`
	Session      AgentSessionPayload           `json:"session"`
	Task         AgentTaskContextPayload       `json:"task"`
	Capabilities AgentCapabilitySectionPayload `json:"capabilities"`
	Limits       AgentLimitsPayload            `json:"limits"`
	Provenance   AgentContextProvenancePayload `json:"provenance"`
}

// AgentSpawnRequest asks the daemon to create a narrowed child session.
type AgentSpawnRequest struct {
	AgentName        string                       `json:"agent_name"`
	Provider         string                       `json:"provider,omitempty"`
	Model            string                       `json:"model,omitempty"`
	ReasoningEffort  ReasoningEffort              `json:"reasoning_effort,omitempty"`
	Speed            Speed                        `json:"speed,omitempty"`
	ACPOptions       []AgentACPOptionSelection    `json:"acp_options,omitempty"`
	Name             string                       `json:"name,omitempty"`
	Workspace        string                       `json:"workspace,omitempty"`
	PromptOverlay    string                       `json:"prompt_overlay,omitempty"`
	SpawnRole        string                       `json:"spawn_role"`
	TTLSeconds       int64                        `json:"ttl_seconds"`
	AutoStopOnParent bool                         `json:"auto_stop_on_parent"`
	NotifyCreator    *bool                        `json:"notify_creator,omitzero"`
	Permissions      SpawnPermissionPolicyPayload `json:"permissions"`
	IdempotencyKey   string                       `json:"idempotency_key,omitempty"`
}

// AgentSpawnPayload is the safe spawn response projection.
type AgentSpawnPayload struct {
	Session     SessionPayload               `json:"session"`
	Lineage     SessionLineagePayload        `json:"lineage"`
	Permissions SpawnPermissionPolicyPayload `json:"permissions"`
}
