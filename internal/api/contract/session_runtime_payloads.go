package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

const (
	contractDirectKey = "direct"
)

// CreateSessionRequest is the shared session creation request payload.
type CreateSessionRequest struct {
	AgentName            string                 `json:"agent_name,omitempty"`
	Provider             string                 `json:"provider,omitempty"`
	Model                string                 `json:"model,omitempty"`
	ReasoningEffort      ReasoningEffort        `json:"reasoning_effort,omitempty"`
	Prompt               string                 `json:"prompt,omitempty"`
	Name                 string                 `json:"name,omitempty"`
	Workspace            string                 `json:"workspace,omitempty"`
	WorkspacePath        string                 `json:"workspace_path,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

// ApproveSessionRequest is the interactive permission approval payload.
type ApproveSessionRequest struct {
	RequestID string `json:"request_id"`
	TurnID    string `json:"turn_id"`
	Decision  string `json:"decision"`
}

// SessionPayload is the shared session response payload.
type SessionPayload struct {
	ID                           string              `json:"id"`
	Name                         string              `json:"name,omitempty"`
	AgentName                    string              `json:"agent_name"`
	Provider                     string              `json:"provider"`
	Model                        string              `json:"model,omitempty"`
	ReasoningEffort              ReasoningEffort     `json:"reasoning_effort,omitempty"`
	WorkspaceID                  string              `json:"workspace_id,omitempty"`
	WorkspacePath                string              `json:"workspace_path,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation,omitempty"`
	Type                         session.Type        `json:"type,omitempty"`
	State                        session.State       `json:"state"`
	Badge                        session.Badge       `json:"badge"`
	Attachable                   bool                `json:"attachable"`
	AttachedTo                   string              `json:"attached_to,omitempty"`
	AttachExpiresAt              *time.Time          `json:"attach_expires_at,omitempty"`
	TranscriptEpoch              int64               `json:"transcript_epoch,omitempty"`
	// StopReason is the session-level stop classification, distinct from AgentEventPayload.StopReason.
	StopReason store.StopReason `json:"stop_reason,omitempty"`
	// StopDetail is the session-level stop context paired with StopReason.
	StopDetail        string                       `json:"stop_detail,omitempty"`
	Failure           *SessionFailurePayload       `json:"failure,omitempty"`
	ACPSessionID      string                       `json:"acp_session_id,omitempty"`
	ACPCaps           *ACPCapsPayload              `json:"acp_caps,omitempty"`
	AvailableCommands []ACPAvailableCommandPayload `json:"available_commands"`
	Activity          *RuntimeActivityPayload      `json:"activity,omitempty"`
	Sandbox           *SessionSandboxPayload       `json:"sandbox,omitempty"`
	Lineage           *SessionLineagePayload       `json:"lineage,omitempty"`
	Health            *SessionHealthPayload        `json:"health,omitempty"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
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
