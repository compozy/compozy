package heartbeat

import (
	"encoding/json"
	"errors"

	"time"
)

const snapshotSchemaVersion = 1

var (
	// ErrSnapshotNotFound reports a missing persisted Heartbeat snapshot.
	ErrSnapshotNotFound = errors.New("heartbeat: snapshot not found")
	// ErrRevisionNotFound reports a missing persisted Heartbeat authoring revision.
	ErrRevisionNotFound = errors.New("heartbeat: revision not found")
	// ErrSessionHealthNotFound reports a missing persisted session health row.
	ErrSessionHealthNotFound = errors.New("heartbeat: session health not found")
	// ErrWakeStateNotFound reports a missing persisted Heartbeat wake state row.
	ErrWakeStateNotFound = errors.New("heartbeat: wake state not found")
	// ErrWakeEventNotFound reports a missing persisted Heartbeat wake event row.
	ErrWakeEventNotFound = errors.New("heartbeat: wake event not found")
	// ErrInvalidSnapshot reports a malformed persisted Heartbeat snapshot.
	ErrInvalidSnapshot = errors.New("heartbeat: invalid snapshot")
	// ErrInvalidRevision reports a malformed Heartbeat authoring revision.
	ErrInvalidRevision = errors.New("heartbeat: invalid revision")
	// ErrInvalidSessionHealth reports a malformed persisted session health row.
	ErrInvalidSessionHealth = errors.New("heartbeat: invalid session health")
	// ErrInvalidWakeState reports a malformed persisted Heartbeat wake state row.
	ErrInvalidWakeState = errors.New("heartbeat: invalid wake state")
	// ErrInvalidWakeEvent reports a malformed persisted Heartbeat wake event row.
	ErrInvalidWakeEvent = errors.New("heartbeat: invalid wake event")
)

// RevisionOperation describes a managed HEARTBEAT.md authoring mutation.
type RevisionOperation string

const (
	// RevisionOperationWrite records a create or update mutation.
	RevisionOperationWrite RevisionOperation = "write"
	// RevisionOperationDelete records a managed delete mutation.
	RevisionOperationDelete RevisionOperation = "delete"
	// RevisionOperationRollback records a managed rollback mutation.
	RevisionOperationRollback RevisionOperation = "rollback"
)

// ActorKind describes the redacted authoring actor class.
type ActorKind string

const (
	ActorKindUser      ActorKind = "user"
	ActorKindAgent     ActorKind = "agent"
	ActorKindExtension ActorKind = "extension"
	ActorKindSystem    ActorKind = "system"
)

// SessionHealthState describes daemon-owned session runtime state for wake eligibility.
type SessionHealthState string

const (
	SessionHealthStateIdle      SessionHealthState = "idle"
	SessionHealthStatePrompting SessionHealthState = "prompting"
	SessionHealthStateStopped   SessionHealthState = "stopped"
	SessionHealthStateDetached  SessionHealthState = "detached"
)

// SessionHealthStatus classifies metadata-only session health.
type SessionHealthStatus string

const (
	SessionHealthHealthy  SessionHealthStatus = "healthy"
	SessionHealthDegraded SessionHealthStatus = "degraded"
	SessionHealthStale    SessionHealthStatus = "stale"
	SessionHealthDead     SessionHealthStatus = "dead"
	SessionHealthUnknown  SessionHealthStatus = "unknown"
)

// SessionHealthIneligibilityReason is a closed reason for wake-ineligible health rows.
type SessionHealthIneligibilityReason string

const (
	SessionHealthReasonPromptActive  SessionHealthIneligibilityReason = "session_prompt_active"
	SessionHealthReasonNotAttachable SessionHealthIneligibilityReason = "session_not_attachable"
	SessionHealthReasonUnhealthy     SessionHealthIneligibilityReason = "session_unhealthy"
	SessionHealthReasonStale         SessionHealthIneligibilityReason = "session_health_stale"
	SessionHealthReasonHung          SessionHealthIneligibilityReason = "session_health_hung"
	SessionHealthReasonDead          SessionHealthIneligibilityReason = "session_health_dead"
	SessionHealthReasonUnknown       SessionHealthIneligibilityReason = "session_health_unknown"
)

// WakeSource classifies who requested an advisory Heartbeat wake.
type WakeSource string

const (
	WakeSourceScheduler      WakeSource = "scheduler"
	WakeSourceManual         WakeSource = "manual"
	WakeSourceHarnessReentry WakeSource = "harness_reentry"
)

// WakeResult classifies the outcome of one advisory Heartbeat wake attempt.
type WakeResult string

const (
	WakeResultSent        WakeResult = "sent"
	WakeResultSkipped     WakeResult = "skipped"
	WakeResultCoalesced   WakeResult = "coalesced"
	WakeResultRateLimited WakeResult = "rate_limited"
	WakeResultFailed      WakeResult = "failed"
)

// WakeReason is a closed deterministic reason for Heartbeat wake state and audit.
type WakeReason string

const (
	WakeReasonSent                  WakeReason = "wake_sent"
	WakeReasonHeartbeatDisabled     WakeReason = "heartbeat_disabled"
	WakeReasonHeartbeatInvalid      WakeReason = "heartbeat_invalid"
	WakeReasonHeartbeatNoPolicy     WakeReason = "heartbeat_no_policy"
	WakeReasonHeartbeatRateLimited  WakeReason = "heartbeat_rate_limited"
	WakeReasonHeartbeatNoEligible   WakeReason = "heartbeat_no_eligible_session"
	WakeReasonCooldownActive        WakeReason = "cooldown_active"
	WakeReasonQuietWindow           WakeReason = "quiet_window"
	WakeReasonSessionNotFound       WakeReason = "session_not_found"
	WakeReasonSessionUnhealthy      WakeReason = "session_unhealthy"
	WakeReasonSessionNotAttachable  WakeReason = "session_not_attachable"
	WakeReasonSessionPromptActive   WakeReason = "session_prompt_active"
	WakeReasonSessionPromptRace     WakeReason = "session_prompt_active_race"
	WakeReasonSyntheticPromptFailed WakeReason = "synthetic_prompt_failed"
	WakeReasonCoalesced             WakeReason = "wake_coalesced"
)

// Snapshot is the immutable storage row for a resolved HEARTBEAT.md policy.
type Snapshot struct {
	ID              string
	WorkspaceID     string
	AgentName       string
	SourcePath      string
	SchemaVersion   int
	Digest          string
	ConfigDigest    string
	Body            string
	FrontmatterJSON json.RawMessage
	ResolvedJSON    json.RawMessage
	DiagnosticsJSON json.RawMessage
	CreatedAt       time.Time
}

// SnapshotEnvelope is the structured JSON envelope stored in Snapshot.ResolvedJSON.
type SnapshotEnvelope struct {
	SchemaVersion    int                `json:"schema_version"`
	Present          bool               `json:"present"`
	Active           bool               `json:"active"`
	Valid            bool               `json:"valid"`
	Summary          string             `json:"summary,omitempty"`
	GuidanceMarkdown string             `json:"guidance_markdown,omitempty"`
	Preferences      Preferences        `json:"preferences"`
	ConfigProvenance ConfigProvenance   `json:"config_provenance"`
	Prompt           PromptContribution `json:"prompt"`
	Status           StatusData         `json:"status"`
	Diagnostics      []Diagnostic       `json:"diagnostics,omitempty"`
}

// SnapshotListQuery filters persisted Heartbeat snapshot rows.
type SnapshotListQuery struct {
	WorkspaceID string
	AgentName   string
	Digest      string
	Limit       int
}

// Revision is one append-only managed HEARTBEAT.md authoring history row.
type Revision struct {
	ID             string
	WorkspaceID    string
	AgentName      string
	SourcePath     string
	Operation      RevisionOperation
	PreviousDigest string
	NewDigest      string
	NewSnapshotID  string
	Body           string
	ActorKind      ActorKind
	ActorID        string
	CreatedAt      time.Time
}

// RevisionListQuery filters managed Heartbeat authoring revision history.
type RevisionListQuery struct {
	WorkspaceID string
	AgentName   string
	Operation   RevisionOperation
	Limit       int
}

// RollbackLookup selects the prior revision body used by managed rollback.
type RollbackLookup struct {
	WorkspaceID string
	AgentName   string
	RevisionID  string
}

// SessionHealth is the metadata-only runtime health row for one session.
type SessionHealth struct {
	SessionID           string
	WorkspaceID         string
	AgentName           string
	State               SessionHealthState
	Health              SessionHealthStatus
	ActivePrompt        bool
	Attachable          bool
	EligibleForWake     bool
	IneligibilityReason string
	LastActivityAt      time.Time
	LastPresenceAt      time.Time
	LastError           string
	UpdatedAt           time.Time
}

// SessionHealthListQuery filters persisted session health rows.
type SessionHealthListQuery struct {
	WorkspaceID     string
	AgentName       string
	SessionID       string
	State           SessionHealthState
	Health          SessionHealthStatus
	EligibleForWake *bool
	Limit           int
}

// WakeState is the per-session cooldown/coalescing summary for Heartbeat wakes.
type WakeState struct {
	WorkspaceID      string
	AgentName        string
	SessionID        string
	PolicySnapshotID string
	LastWakeAt       time.Time
	NextAllowedAt    time.Time
	CoalescedCount   int
	LastResult       WakeResult
	LastReason       WakeReason
	UpdatedAt        time.Time
}

// WakeStateListQuery filters Heartbeat wake state rows.
type WakeStateListQuery struct {
	WorkspaceID string
	AgentName   string
	SessionID   string
	Limit       int
}

// WakeEvent is one retained Heartbeat wake audit row.
type WakeEvent struct {
	ID                string
	WorkspaceID       string
	AgentName         string
	SessionID         string
	PolicySnapshotID  string
	Source            WakeSource
	Result            WakeResult
	Reason            WakeReason
	SyntheticPromptID string
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

// WakeEventListQuery filters retained Heartbeat wake audit rows.
type WakeEventListQuery struct {
	WorkspaceID string
	AgentName   string
	SessionID   string
	Source      WakeSource
	Result      WakeResult
	Reason      WakeReason
	Limit       int
}
