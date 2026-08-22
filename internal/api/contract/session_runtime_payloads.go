package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	contractDirectKey = "direct"

	// SessionAttachDefaultTTLSeconds is the lease duration used when attach omits a positive TTL.
	SessionAttachDefaultTTLSeconds = 15 * 60
	// SessionAttachMaxTTLSeconds is the largest attach lease accepted by the daemon.
	SessionAttachMaxTTLSeconds = 24 * 60 * 60
)

// CreateSessionRequest is the shared session creation request payload.
type CreateSessionRequest struct {
	AgentName     string                     `json:"agent_name,omitempty"`
	Name          string                     `json:"name,omitempty"`
	Workspace     string                     `json:"workspace,omitempty"`
	WorkspacePath string                     `json:"workspace_path,omitempty"`
	Worktree      string                     `json:"worktree,omitempty"`
	NewWorktree   *NewSessionWorktreeRequest `json:"new_worktree,omitempty"`
	// ParentSessionID records creation provenance; the parent must live in the
	// target workspace and the link never narrows the child's lifecycle.
	ParentSessionID      string                 `json:"parent_session_id,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

// RenameSessionRequest changes the durable display name of one user session.
type RenameSessionRequest struct {
	Name string `json:"name"`
}

// NewSessionWorktreeRequest materializes one worktree before the session starts.
type NewSessionWorktreeRequest struct {
	Name string `json:"name,omitempty"`
}

// ForkSessionWorktreeRequest creates a clean child session in one worktree.
// Confirmed is mandatory because the origin session is never moved in place.
type ForkSessionWorktreeRequest struct {
	Worktree    string                     `json:"worktree,omitempty"`
	NewWorktree *NewSessionWorktreeRequest `json:"new_worktree,omitempty"`
	Confirmed   bool                       `json:"confirmed"`
}

// ApproveSessionRequest is the interactive permission approval payload.
type ApproveSessionRequest struct {
	RequestID string `json:"request_id"`
	TurnID    string `json:"turn_id"`
	Decision  string `json:"decision"`
}

// SessionPayload is the shared session response payload.
type SessionPayload struct {
	ID                           string                      `json:"id"`
	ProfileID                    string                      `json:"profile_id"`
	ProfileName                  string                      `json:"profile_name"`
	ProfileColor                 string                      `json:"profile_color,omitempty"`
	ProfileIcon                  string                      `json:"profile_icon,omitempty"`
	Name                         string                      `json:"name,omitempty"`
	AgentName                    string                      `json:"agent_name"`
	Runtime                      SessionRuntimePayload       `json:"runtime"`
	WorkspaceID                  string                      `json:"workspace_id,omitempty"`
	WorkspacePath                string                      `json:"workspace_path,omitempty"`
	WorktreeID                   string                      `json:"worktree_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec         `json:"resolved_network_participation,omitempty"`
	Type                         session.Type                `json:"type,omitempty"`
	State                        session.State               `json:"state"`
	Badge                        session.Badge               `json:"badge"`
	Attachable                   bool                        `json:"attachable"`
	AttachedTo                   string                      `json:"attached_to,omitempty"`
	AttachExpiresAt              *time.Time                  `json:"attach_expires_at,omitempty"`
	TranscriptEpoch              int64                       `json:"transcript_epoch,omitempty"`
	AttentionChangedAt           *time.Time                  `json:"attention_changed_at,omitempty"`
	PendingInteractions          []PendingInteractionPayload `json:"pending_interactions"`
	ArchivedAt                   *time.Time                  `json:"archived_at"`
	// StopReason is the session-level stop classification, distinct from AgentEventPayload.StopReason.
	StopReason store.StopReason `json:"stop_reason,omitempty"`
	// StopDetail is the session-level stop context paired with StopReason.
	StopDetail        string                       `json:"stop_detail,omitempty"`
	Failure           *SessionFailurePayload       `json:"failure,omitempty"`
	AvailableCommands []ACPAvailableCommandPayload `json:"available_commands"`
	Activity          *RuntimeActivityPayload      `json:"activity,omitempty"`
	Sandbox           *SessionSandboxPayload       `json:"sandbox,omitempty"`
	Lineage           *SessionLineagePayload       `json:"lineage,omitempty"`
	Health            *SessionHealthPayload        `json:"health,omitempty"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
}

// SessionRuntimePayload reports the current ACP binding and its effective runtime.
type SessionRuntimePayload struct {
	Status            session.RuntimeStatus             `json:"status"`
	Transition        session.RuntimeTransitionStrategy `json:"transition,omitempty"`
	Failure           string                            `json:"failure,omitempty"`
	Selected          *PromptRuntimeSelectionPayload    `json:"selected,omitempty"`
	SelectionRevision int64                             `json:"selection_revision"`
	Effective         *RuntimeSelectionPayload          `json:"effective,omitempty"`
	ACPSessionID      string                            `json:"acp_session_id,omitempty"`
	ACPCaps           *ACPCapsPayload                   `json:"acp_caps,omitempty"`
}

// SetSessionRuntimeRequest durably selects the default runtime for future prompts.
type SetSessionRuntimeRequest struct {
	Runtime          PromptRuntimeSelectionPayload `json:"runtime"`
	ExpectedRevision *int64                        `json:"expected_revision"`
}

// RuntimeSelectionPayload is the effective runtime bound to a logical session.
type RuntimeSelectionPayload struct {
	Provider        string           `json:"provider"`
	Model           string           `json:"model,omitempty"`
	ReasoningEffort ReasoningEffort  `json:"reasoning_effort,omitempty"`
	Speed           Speed            `json:"speed,omitempty"`
	SpeedResolution *SpeedResolution `json:"speed_resolution,omitempty"`
}

// PromptRuntimeSelectionPayload is the caller-selected runtime for one prompt.
type PromptRuntimeSelectionPayload struct {
	Provider        string          `json:"provider"`
	Model           string          `json:"model,omitempty"`
	ReasoningEffort ReasoningEffort `json:"reasoning_effort,omitempty"`
	Speed           Speed           `json:"speed,omitempty"`
}

// SessionFailurePayload is the redacted lifecycle failure diagnostic shared by
// session read paths, event streams, and health summaries.
type SessionFailurePayload struct {
	Kind            store.FailureKind `json:"kind"`
	Summary         string            `json:"summary,omitempty"`
	CrashBundlePath string            `json:"crash_bundle_path,omitempty"`
}

// RuntimeActivityPayload is the shared JSON representation of active prompt supervision state.
type RuntimeActivityPayload struct {
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
}

// AttachSessionRequest captures explicit attach-lock options.
type AttachSessionRequest struct {
	AttachedTo string `json:"attached_to,omitempty"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
}

// SessionAttachPayload reports the attach lease acquired by one caller.
type SessionAttachPayload struct {
	SessionID       string    `json:"session_id"`
	AttachedTo      string    `json:"attached_to"`
	AttachExpiresAt time.Time `json:"attach_expires_at"`
	AttachedAt      time.Time `json:"attached_at"`
}

// TranscriptMarkerPayload is the typed transcript marker shape shared by logs,
// recap, and transcript UI projections.
type TranscriptMarkerPayload struct {
	Kind       string          `json:"kind"`
	OccurredAt time.Time       `json:"occurred_at"`
	Summary    string          `json:"summary"`
	Evidence   map[string]any  `json:"evidence,omitempty"`
	Diagnostic json.RawMessage `json:"diagnostic,omitempty"`
}

// RecapSnapshotPayload records the consistent read boundary for one recap.
type RecapSnapshotPayload struct {
	GeneratedAt      time.Time `json:"generated_at"`
	EventCursor      int64     `json:"event_cursor"`
	TranscriptCursor int64     `json:"transcript_cursor"`
	QueueGeneration  int64     `json:"queue_generation"`
	Consistency      string    `json:"consistency"`
}

// RecapPayload is a deterministic session recap composed from persisted daemon state.
type RecapPayload struct {
	Session        SessionPayload            `json:"session"`
	ActiveRun      *TaskRunPayload           `json:"active_run,omitempty"`
	RecentMarkers  []TranscriptMarkerPayload `json:"recent_markers"`
	RecentMessages []transcript.UIMessage    `json:"recent_messages"`
	PendingInputs  int                       `json:"pending_inputs"`
	PendingMarkers int                       `json:"pending_markers"`
	Snapshot       RecapSnapshotPayload      `json:"snapshot"`
}

// SessionSandboxPayload is the shared session sandbox response payload.
type SessionSandboxPayload struct {
	SandboxID         string          `json:"sandbox_id,omitempty"`
	Backend           string          `json:"backend,omitempty"`
	Profile           string          `json:"profile,omitempty"`
	State             string          `json:"state,omitempty"`
	InstanceID        string          `json:"instance_id,omitempty"`
	LastSyncError     string          `json:"last_sync_error,omitempty"`
	ProviderStateJSON json.RawMessage `json:"provider_state_json,omitempty"`
}
