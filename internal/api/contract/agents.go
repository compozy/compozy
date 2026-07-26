package contract

import (
	"encoding/json"
	"errors"

	"time"

	"github.com/compozy/agh/internal/network/participation"

	taskpkg "github.com/compozy/agh/internal/task"
)

// CoordinationMessageKind identifies the MVP task-run coordination message kind.
type CoordinationMessageKind string

const (
	// CoordinationMessageStatus reports non-authoritative work progress.
	CoordinationMessageStatus CoordinationMessageKind = "status"
	// CoordinationMessageRequest asks another participant for information or action.
	CoordinationMessageRequest CoordinationMessageKind = "request"
	// CoordinationMessageReply answers a prior coordination message.
	CoordinationMessageReply CoordinationMessageKind = "reply"
	// CoordinationMessageBlocker reports a blocking condition for coordinated work.
	CoordinationMessageBlocker CoordinationMessageKind = "blocker"
	// CoordinationMessageHandoff transfers conversational context between participants.
	CoordinationMessageHandoff CoordinationMessageKind = "handoff"
	// CoordinationMessageResult shares task-run output before or after terminal task APIs.
	CoordinationMessageResult CoordinationMessageKind = "result"
	// CoordinationMessageReviewRequest asks for review of coordinated work.
	CoordinationMessageReviewRequest CoordinationMessageKind = "review_request"
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

var (
	// ErrRawClaimTokenMetadata reports an unsafe raw lease credential in channel metadata.
	ErrRawClaimTokenMetadata = errors.New("contract: coordination metadata must not contain raw lease credentials")
	// ErrInvalidCoordinationMessageMetadata reports missing or invalid typed correlation metadata.
	ErrInvalidCoordinationMessageMetadata = errors.New("contract: invalid coordination message metadata")
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
	TTLExpiresAt     *time.Time                   `json:"ttl_expires_at,omitempty"`
	AutoStopOnParent bool                         `json:"auto_stop_on_parent"`
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
	Tools           []string `json:"tools"`
	Skills          []string `json:"skills"`
	MCPServers      []string `json:"mcp_servers"`
	WorkspacePaths  []string `json:"workspace_paths"`
	NetworkChannels []string `json:"network_channels"`
	SandboxProfiles []string `json:"sandbox_profiles"`
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

// CoordinationChannelPayload describes the stable task-run coordination channel binding.
type CoordinationChannelPayload struct {
	ID                  string                    `json:"id"`
	DisplayName         string                    `json:"display_name"`
	Purpose             string                    `json:"purpose,omitempty"`
	WorkspaceID         string                    `json:"workspace_id,omitempty"`
	TaskID              string                    `json:"task_id,omitempty"`
	RunID               string                    `json:"run_id,omitempty"`
	WorkflowID          string                    `json:"workflow_id,omitempty"`
	AllowedMessageKinds []CoordinationMessageKind `json:"allowed_message_kinds"`
	LastActivityAt      *time.Time                `json:"last_activity_at,omitempty"`
}

// TaskRunLeaseSummaryPayload is the safe read projection for task-run lease state.
type TaskRunLeaseSummaryPayload struct {
	TaskID                       string                      `json:"task_id"`
	RunID                        string                      `json:"run_id"`
	Status                       taskpkg.RunStatus           `json:"status"`
	SessionID                    string                      `json:"session_id,omitempty"`
	ClaimedBy                    *taskpkg.ActorIdentity      `json:"claimed_by,omitempty"`
	ClaimTokenHash               string                      `json:"claim_token_hash,omitempty"`
	LeaseUntil                   *time.Time                  `json:"lease_until,omitempty"`
	HeartbeatAt                  *time.Time                  `json:"heartbeat_at,omitempty"`
	ResolvedNetworkParticipation *participation.Spec         `json:"resolved_network_participation,omitempty"`
	CoordinationChannel          *CoordinationChannelPayload `json:"coordination_channel,omitempty"`
}

// AgentTaskContextPayload is the bounded active-task section in `/agent/context`.
type AgentTaskContextPayload struct {
	Available bool                        `json:"available"`
	Task      *TaskReferencePayload       `json:"task,omitempty"`
	Lease     *TaskRunLeaseSummaryPayload `json:"lease,omitempty"`
	Bundle    *taskpkg.ContextBundle      `json:"bundle,omitempty"`
}

// AgentCoordinationChannelContextPayload is the active coordination-channel section.
type AgentCoordinationChannelContextPayload struct {
	Available bool                        `json:"available"`
	Channel   *CoordinationChannelPayload `json:"channel,omitempty"`
}

// AgentContextSectionMetaPayload reports bounding/truncation metadata for context sections.
type AgentContextSectionMetaPayload struct {
	Limit     int  `json:"limit"`
	Returned  int  `json:"returned"`
	Truncated bool `json:"truncated"`
}

// AgentInboxItemPayload is one compact inbox item in the bounded agent context.
type AgentInboxItemPayload struct {
	MessageID string                             `json:"message_id"`
	ChannelID string                             `json:"channel_id"`
	Kind      CoordinationMessageKind            `json:"kind"`
	Metadata  CoordinationMessageMetadataPayload `json:"metadata"`
	Preview   string                             `json:"preview,omitempty"`
	Timestamp time.Time                          `json:"timestamp"`
}

// AgentInboxSummaryPayload is the bounded inbox section in `/agent/context`.
type AgentInboxSummaryPayload struct {
	Section     AgentContextSectionMetaPayload `json:"section"`
	UnreadCount int                            `json:"unread_count"`
	Items       []AgentInboxItemPayload        `json:"items"`
}

// AgentPeerSummaryPayload is one compact peer entry in the bounded agent context.
type AgentPeerSummaryPayload struct {
	PeerID       string   `json:"peer_id"`
	SessionID    string   `json:"session_id,omitempty"`
	DisplayName  string   `json:"display_name,omitempty"`
	ChannelID    string   `json:"channel_id,omitempty"`
	Capabilities []string `json:"capabilities"`
}

// AgentPeerRosterPayload is the bounded peer roster section in `/agent/context`.
type AgentPeerRosterPayload struct {
	Section AgentContextSectionMetaPayload `json:"section"`
	Peers   []AgentPeerSummaryPayload      `json:"peers"`
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
	Channels         []CoordinationChannelPayload `json:"channels"`
	ActiveTaskLeases []TaskRunLeaseSummaryPayload `json:"active_task_leases"`
	Coordinator      CoordinatorConfigPayload     `json:"coordinator"`
	Limits           AgentLimitsPayload           `json:"limits"`
}

// AgentContextPayload is the stable bounded situation payload returned by `/agent/context`.
type AgentContextPayload struct {
	Self                AgentIdentityPayload                   `json:"self"`
	Workspace           AgentWorkspacePayload                  `json:"workspace"`
	Session             AgentSessionPayload                    `json:"session"`
	Soul                AgentSoulSectionPayload                `json:"soul"`
	Task                AgentTaskContextPayload                `json:"task"`
	CoordinationChannel AgentCoordinationChannelContextPayload `json:"coordination_channel"`
	InboxSummary        AgentInboxSummaryPayload               `json:"inbox_summary"`
	PeerRoster          AgentPeerRosterPayload                 `json:"peer_roster"`
	Capabilities        AgentCapabilitySectionPayload          `json:"capabilities"`
	Limits              AgentLimitsPayload                     `json:"limits"`
	Provenance          AgentContextProvenancePayload          `json:"provenance"`
}

// CoordinationMessageMetadataPayload carries typed task/run correlation for channel messages.
type CoordinationMessageMetadataPayload struct {
	TaskID        string                     `json:"task_id"`
	RunID         string                     `json:"run_id"`
	WorkflowID    string                     `json:"workflow_id,omitempty"`
	ChannelID     string                     `json:"channel_id"`
	MessageKind   CoordinationMessageKind    `json:"message_kind"`
	CorrelationID string                     `json:"correlation_id"`
	Ext           map[string]json.RawMessage `json:"ext,omitempty"`
}

// AgentChannelSendRequest sends one task-bound coordination message.
type AgentChannelSendRequest struct {
	Body           json.RawMessage                    `json:"body"`
	Metadata       CoordinationMessageMetadataPayload `json:"metadata"`
	IdempotencyKey string                             `json:"idempotency_key,omitempty"`
}

// AgentChannelReplyRequest replies to one delivered coordination message.
type AgentChannelReplyRequest struct {
	ReplyToMessageID string                             `json:"reply_to_message_id"`
	Body             json.RawMessage                    `json:"body"`
	Metadata         CoordinationMessageMetadataPayload `json:"metadata"`
	IdempotencyKey   string                             `json:"idempotency_key,omitempty"`
}

// AgentChannelMessagePayload is one safe channel message read projection.
type AgentChannelMessagePayload struct {
	MessageID     string                             `json:"message_id"`
	ChannelID     string                             `json:"channel_id"`
	FromSessionID string                             `json:"from_session_id"`
	ToSessionID   string                             `json:"to_session_id,omitempty"`
	Body          json.RawMessage                    `json:"body"`
	Metadata      CoordinationMessageMetadataPayload `json:"metadata"`
	Timestamp     time.Time                          `json:"timestamp"`
}

// AgentSpawnRequest asks the daemon to create a narrowed child session.
type AgentSpawnRequest struct {
	AgentName        string                       `json:"agent_name"`
	Provider         string                       `json:"provider,omitempty"`
	Model            string                       `json:"model,omitempty"`
	Name             string                       `json:"name,omitempty"`
	PromptOverlay    string                       `json:"prompt_overlay,omitempty"`
	SpawnRole        string                       `json:"spawn_role"`
	TTLSeconds       int64                        `json:"ttl_seconds"`
	AutoStopOnParent bool                         `json:"auto_stop_on_parent"`
	Permissions      SpawnPermissionPolicyPayload `json:"permissions"`
	IdempotencyKey   string                       `json:"idempotency_key,omitempty"`
}

// AgentSpawnPayload is the safe spawn response projection.
type AgentSpawnPayload struct {
	Session     SessionPayload               `json:"session"`
	Lineage     SessionLineagePayload        `json:"lineage"`
	Permissions SpawnPermissionPolicyPayload `json:"permissions"`
}
