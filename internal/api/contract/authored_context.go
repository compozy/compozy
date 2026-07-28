package contract

import (
	"errors"

	"time"
)

const maxAuthoredContextLastErrorBytes = 2048

var (
	// ErrUnsafeAuthoredContextPayload reports raw credentials or disallowed prompt data in a public DTO.
	ErrUnsafeAuthoredContextPayload = errors.New(
		"contract: authored context payload contains unsafe token or prompt data",
	)
	// ErrInvalidAuthoredContextEnum reports a value outside a closed public contract enum.
	ErrInvalidAuthoredContextEnum = errors.New("contract: invalid authored context enum")
)

// AuthoredValidationStatus is the closed validation state shared by authored-context surfaces.
type AuthoredValidationStatus string

const (
	AuthoredValidationMissing  AuthoredValidationStatus = "missing"
	AuthoredValidationInactive AuthoredValidationStatus = "inactive"
	AuthoredValidationValid    AuthoredValidationStatus = "valid"
	AuthoredValidationInvalid  AuthoredValidationStatus = "invalid"
)

// AuthoredDiagnosticSeverity is the closed diagnostic severity shared by Soul and Heartbeat DTOs.
type AuthoredDiagnosticSeverity string

const (
	AuthoredDiagnosticInfo    AuthoredDiagnosticSeverity = "info"
	AuthoredDiagnosticWarning AuthoredDiagnosticSeverity = "warning"
	AuthoredDiagnosticError   AuthoredDiagnosticSeverity = "error"
)

// AgentSoulRevisionAction describes one managed SOUL.md authoring mutation.
type AgentSoulRevisionAction string

const (
	AgentSoulRevisionPut      AgentSoulRevisionAction = "put"
	AgentSoulRevisionDelete   AgentSoulRevisionAction = "delete"
	AgentSoulRevisionRollback AgentSoulRevisionAction = "rollback"
)

// HeartbeatRevisionOperation describes one managed HEARTBEAT.md authoring mutation.
type HeartbeatRevisionOperation string

const (
	HeartbeatRevisionWrite    HeartbeatRevisionOperation = "write"
	HeartbeatRevisionDelete   HeartbeatRevisionOperation = "delete"
	HeartbeatRevisionRollback HeartbeatRevisionOperation = "rollback"
)

// HeartbeatActorKind describes a redacted Heartbeat authoring actor class.
type HeartbeatActorKind string

const (
	HeartbeatActorUser      HeartbeatActorKind = "user"
	HeartbeatActorAgent     HeartbeatActorKind = "agent"
	HeartbeatActorExtension HeartbeatActorKind = "extension"
	HeartbeatActorSystem    HeartbeatActorKind = "system"
)

// SessionHealthState describes daemon-owned runtime state for wake eligibility.
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

// SessionHealthIneligibilityReason is the closed reason for wake-ineligible health rows.
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

// HeartbeatWakeSource classifies who requested an advisory Heartbeat wake.
type HeartbeatWakeSource string

const (
	HeartbeatWakeSourceScheduler      HeartbeatWakeSource = "scheduler"
	HeartbeatWakeSourceManual         HeartbeatWakeSource = "manual"
	HeartbeatWakeSourceHarnessReentry HeartbeatWakeSource = "harness_reentry"
)

// HeartbeatWakeResult classifies the outcome of an advisory Heartbeat wake attempt.
type HeartbeatWakeResult string

const (
	HeartbeatWakeResultSent        HeartbeatWakeResult = "sent"
	HeartbeatWakeResultSkipped     HeartbeatWakeResult = "skipped"
	HeartbeatWakeResultCoalesced   HeartbeatWakeResult = "coalesced"
	HeartbeatWakeResultRateLimited HeartbeatWakeResult = "rate_limited"
	HeartbeatWakeResultFailed      HeartbeatWakeResult = "failed"
)

// HeartbeatWakeReason is the closed deterministic reason for Heartbeat wake audit.
type HeartbeatWakeReason string

const (
	HeartbeatWakeReasonSent                  HeartbeatWakeReason = "wake_sent"
	HeartbeatWakeReasonDisabled              HeartbeatWakeReason = "heartbeat_disabled"
	HeartbeatWakeReasonInvalid               HeartbeatWakeReason = "heartbeat_invalid"
	HeartbeatWakeReasonNoPolicy              HeartbeatWakeReason = "heartbeat_no_policy"
	HeartbeatWakeReasonRateLimited           HeartbeatWakeReason = "heartbeat_rate_limited"
	HeartbeatWakeReasonNoEligibleSession     HeartbeatWakeReason = "heartbeat_no_eligible_session"
	HeartbeatWakeReasonCooldownActive        HeartbeatWakeReason = "cooldown_active"
	HeartbeatWakeReasonQuietWindow           HeartbeatWakeReason = "quiet_window"
	HeartbeatWakeReasonSessionNotFound       HeartbeatWakeReason = "session_not_found"
	HeartbeatWakeReasonSessionUnhealthy      HeartbeatWakeReason = "session_unhealthy"
	HeartbeatWakeReasonSessionNotAttachable  HeartbeatWakeReason = "session_not_attachable"
	HeartbeatWakeReasonSessionPromptActive   HeartbeatWakeReason = "session_prompt_active"
	HeartbeatWakeReasonSessionPromptRace     HeartbeatWakeReason = "session_prompt_active_race"
	HeartbeatWakeReasonSyntheticPromptFailed HeartbeatWakeReason = "synthetic_prompt_failed"
	HeartbeatWakeReasonCoalesced             HeartbeatWakeReason = "wake_coalesced"
)

// AuthoredContextDiagnosticPayload is a redacted, deterministic authored-context diagnostic.
type AuthoredContextDiagnosticPayload struct {
	Code         string                     `json:"code"`
	Severity     AuthoredDiagnosticSeverity `json:"severity"`
	Message      string                     `json:"message"`
	Field        string                     `json:"field,omitempty"`
	Section      string                     `json:"section,omitempty"`
	SourcePath   string                     `json:"source_path,omitempty"`
	Line         int                        `json:"line,omitempty"`
	Column       int                        `json:"column,omitempty"`
	OwnerSurface string                     `json:"owner_surface,omitempty"`
}

// AuthoredContextLimitsPayload reports stable parser/projection limits.
type AuthoredContextLimitsPayload struct {
	MaxBodyBytes           int64 `json:"max_body_bytes"`
	ContextProjectionBytes int64 `json:"context_projection_bytes,omitempty"`
	MaxBytes               int64 `json:"max_bytes,omitempty"`
}

// AuthoredContextActorPayload records redacted actor or origin identity.
type AuthoredContextActorPayload struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref,omitempty"`
}

// AgentSoulConfigProvenancePayload reports the Soul config subset that shaped a read model.
type AgentSoulConfigProvenancePayload struct {
	Digest                 string `json:"digest"`
	Source                 string `json:"source,omitempty"`
	Enabled                bool   `json:"enabled"`
	MaxBodyBytes           int64  `json:"max_body_bytes"`
	ContextProjectionBytes int64  `json:"context_projection_bytes"`
}

// AgentSoulFrontmatterPayload is the allowlisted strict SOUL.md metadata projection.
type AgentSoulFrontmatterPayload struct {
	Version       string   `json:"version,omitempty"`
	Role          string   `json:"role,omitempty"`
	Tone          []string `json:"tone,omitempty"`
	Principles    []string `json:"principles,omitempty"`
	Constraints   []string `json:"constraints,omitempty"`
	Collaboration []string `json:"collaboration,omitempty"`
	MemoryPolicy  []string `json:"memory_policy,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

// AgentSoulSectionPayload is the compact, context-safe Soul projection for `/agent/context`.
type AgentSoulSectionPayload struct {
	Enabled          bool                     `json:"enabled"`
	Present          bool                     `json:"present"`
	Active           bool                     `json:"active"`
	Valid            bool                     `json:"valid"`
	ValidationStatus AuthoredValidationStatus `json:"validation_status,omitempty"`
	SnapshotID       string                   `json:"snapshot_id,omitempty"`
	Digest           string                   `json:"digest,omitempty"`
	ConfigDigest     string                   `json:"config_digest,omitempty"`
	SourcePath       string                   `json:"source_path,omitempty"`
	Role             string                   `json:"role,omitempty"`
	Tone             []string                 `json:"tone"`
	Principles       []string                 `json:"principles"`
	Truncated        bool                     `json:"truncated,omitempty"`
	MaxBytes         int64                    `json:"max_bytes,omitempty"`
	MaxBodyBytes     int64                    `json:"max_body_bytes,omitempty"`
}

// AgentSoulPayload is the full resolved Soul read model for dedicated inspect/authoring surfaces.
type AgentSoulPayload struct {
	AgentName        string                             `json:"agent_name,omitempty"`
	Enabled          bool                               `json:"enabled"`
	Present          bool                               `json:"present"`
	Active           bool                               `json:"active"`
	Valid            bool                               `json:"valid"`
	ValidationStatus AuthoredValidationStatus           `json:"validation_status"`
	SnapshotID       string                             `json:"snapshot_id,omitempty"`
	RevisionID       string                             `json:"revision_id,omitempty"`
	Digest           string                             `json:"digest,omitempty"`
	SourcePath       string                             `json:"source_path,omitempty"`
	Frontmatter      AgentSoulFrontmatterPayload        `json:"frontmatter"`
	Body             string                             `json:"body,omitempty"`
	Truncated        bool                               `json:"truncated,omitempty"`
	Limits           AuthoredContextLimitsPayload       `json:"limits"`
	ConfigProvenance AgentSoulConfigProvenancePayload   `json:"config_provenance"`
	Diagnostics      []AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
	CreatedAt        *time.Time                         `json:"created_at,omitempty"`
}

// AgentSoulValidateRequest validates a proposed SOUL.md body or the current file.
type AgentSoulValidateRequest struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	Body        string `json:"body,omitempty"`
}

// AgentSoulValidateByPathRequest validates SOUL.md for the route agent.
type AgentSoulValidateByPathRequest struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Body        string `json:"body,omitempty"`
}

// AgentSoulPutRequest creates or replaces SOUL.md through the managed authoring service.
type AgentSoulPutRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	AgentName      string `json:"agent_name"`
	Body           string `json:"body"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// AgentSoulPutByPathRequest creates or replaces SOUL.md for the route agent.
type AgentSoulPutByPathRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Body           string `json:"body"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// AgentSoulDeleteRequest removes SOUL.md through the managed authoring service.
type AgentSoulDeleteRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	AgentName      string `json:"agent_name"`
	ExpectedDigest string `json:"expected_digest"`
}

// AgentSoulDeleteByPathRequest removes SOUL.md for the route agent.
type AgentSoulDeleteByPathRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ExpectedDigest string `json:"expected_digest"`
}

// AgentSoulRollbackRequest restores a prior SOUL.md revision through the managed authoring service.
type AgentSoulRollbackRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	AgentName      string `json:"agent_name"`
	RevisionID     string `json:"revision_id"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// AgentSoulRollbackByPathRequest restores a prior SOUL.md revision for the route agent.
type AgentSoulRollbackByPathRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	RevisionID     string `json:"revision_id"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// AgentSoulHistoryRequest lists managed SOUL.md authoring revisions.
type AgentSoulHistoryRequest struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name"`
	Limit       int    `json:"limit,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
}

// AgentSoulRevisionPayload is a redacted SOUL.md authoring history row.
type AgentSoulRevisionPayload struct {
	ID             string                             `json:"id"`
	AgentName      string                             `json:"agent_name"`
	SourcePath     string                             `json:"source_path"`
	Action         AgentSoulRevisionAction            `json:"action"`
	PreviousDigest string                             `json:"previous_digest,omitempty"`
	NewDigest      string                             `json:"new_digest,omitempty"`
	Diagnostics    []AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
	Actor          AuthoredContextActorPayload        `json:"actor"`
	Origin         *AuthoredContextActorPayload       `json:"origin,omitempty"`
	CreatedAt      time.Time                          `json:"created_at"`
}

// AgentSoulHistoryResponse returns bounded SOUL.md authoring history.
type AgentSoulHistoryResponse struct {
	Revisions  []AgentSoulRevisionPayload `json:"revisions"`
	NextCursor string                     `json:"next_cursor,omitempty"`
}

// AgentSoulMutationResponse returns the post-mutation read model and audit row.
type AgentSoulMutationResponse struct {
	Soul     AgentSoulPayload         `json:"soul"`
	Revision AgentSoulRevisionPayload `json:"revision"`
}

// SessionSoulRefreshRequest refreshes an idle session Soul snapshot through body-level CAS.
type SessionSoulRefreshRequest struct {
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}
