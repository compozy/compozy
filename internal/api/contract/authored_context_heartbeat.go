package contract

import "time"

// HeartbeatTimeWindowPayload is one authored local wall-clock active or quiet window.
type HeartbeatTimeWindowPayload struct {
	Timezone string `json:"timezone"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

// HeartbeatContextProjectionPayload captures authored context projection hints.
type HeartbeatContextProjectionPayload struct {
	Include []string `json:"include,omitempty"`
}

// HeartbeatFrontmatterPreferencesPayload captures authored preference hints before config bounds.
type HeartbeatFrontmatterPreferencesPayload struct {
	MinInterval  string                       `json:"min_interval,omitempty"`
	ActiveHours  []HeartbeatTimeWindowPayload `json:"active_hours,omitempty"`
	QuietWindows []HeartbeatTimeWindowPayload `json:"quiet_windows,omitempty"`
}

// HeartbeatFrontmatterPayload is the allowlisted strict HEARTBEAT.md metadata projection.
type HeartbeatFrontmatterPayload struct {
	Version     int                                    `json:"version"`
	Enabled     bool                                   `json:"enabled"`
	Summary     string                                 `json:"summary,omitempty"`
	Preferences HeartbeatFrontmatterPreferencesPayload `json:"preferences"`
	Context     HeartbeatContextProjectionPayload      `json:"context"`
}

// HeartbeatPreferencesPayload captures config-bound wake policy hints.
type HeartbeatPreferencesPayload struct {
	MinInterval  string                            `json:"min_interval"`
	ActiveHours  []HeartbeatTimeWindowPayload      `json:"active_hours,omitempty"`
	QuietWindows []HeartbeatTimeWindowPayload      `json:"quiet_windows,omitempty"`
	Context      HeartbeatContextProjectionPayload `json:"context"`
}

// HeartbeatConfigSubsetPayload is the canonical config authority subset for Heartbeat.
type HeartbeatConfigSubsetPayload struct {
	Enabled                      bool   `json:"enabled"`
	MaxBodyBytes                 int64  `json:"max_body_bytes"`
	ContextProjectionBytes       int64  `json:"context_projection_bytes"`
	MinInterval                  string `json:"min_interval"`
	DefaultInterval              string `json:"default_interval"`
	WakeCooldown                 string `json:"wake_cooldown"`
	MaxWakesPerCycle             int    `json:"max_wakes_per_cycle"`
	ActiveSessionOnly            bool   `json:"active_session_only"`
	AllowActiveHoursPreferences  bool   `json:"allow_active_hours_preferences"`
	WakeEventRetention           string `json:"wake_event_retention"`
	SessionHealthStaleAfter      string `json:"session_health_stale_after"`
	SessionHealthHookMinInterval string `json:"session_health_hook_min_interval"`
}

// HeartbeatConfigProvenancePayload reports the Heartbeat config subset that shaped a policy.
type HeartbeatConfigProvenancePayload struct {
	Digest string                       `json:"digest"`
	Subset HeartbeatConfigSubsetPayload `json:"subset"`
}

// HeartbeatPromptContributionPayload is the bounded synthetic-wake prompt contribution.
type HeartbeatPromptContributionPayload struct {
	Active           bool                               `json:"active"`
	Digest           string                             `json:"digest,omitempty"`
	ConfigDigest     string                             `json:"config_digest,omitempty"`
	SourcePath       string                             `json:"source_path,omitempty"`
	Summary          string                             `json:"summary,omitempty"`
	GuidanceMarkdown string                             `json:"guidance_markdown,omitempty"`
	Preferences      HeartbeatPreferencesPayload        `json:"preferences"`
	Truncated        bool                               `json:"truncated"`
	MaxBytes         int64                              `json:"max_bytes"`
	MaxBodyBytes     int64                              `json:"max_body_bytes"`
	Diagnostics      []AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
	Context          HeartbeatContextProjectionPayload  `json:"context"`
}

// HeartbeatPolicyPayload is the full resolved HEARTBEAT.md read model.
type HeartbeatPolicyPayload struct {
	AgentName        string                             `json:"agent_name,omitempty"`
	Enabled          bool                               `json:"enabled"`
	Present          bool                               `json:"present"`
	Active           bool                               `json:"active"`
	Valid            bool                               `json:"valid"`
	ValidationStatus AuthoredValidationStatus           `json:"validation_status"`
	SourcePath       string                             `json:"source_path,omitempty"`
	Digest           string                             `json:"digest,omitempty"`
	ConfigDigest     string                             `json:"config_digest,omitempty"`
	SnapshotID       string                             `json:"snapshot_id,omitempty"`
	SchemaVersion    int                                `json:"schema_version"`
	Summary          string                             `json:"summary,omitempty"`
	GuidanceMarkdown string                             `json:"guidance_markdown,omitempty"`
	Frontmatter      HeartbeatFrontmatterPayload        `json:"frontmatter"`
	Preferences      HeartbeatPreferencesPayload        `json:"preferences"`
	ConfigProvenance HeartbeatConfigProvenancePayload   `json:"config_provenance"`
	Prompt           HeartbeatPromptContributionPayload `json:"prompt"`
	Diagnostics      []AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
	Limits           AuthoredContextLimitsPayload       `json:"limits"`
	CreatedAt        *time.Time                         `json:"created_at,omitempty"`
}

// HeartbeatValidateRequest validates a proposed HEARTBEAT.md body.
type HeartbeatValidateRequest struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	Body        string `json:"body"`
}

// HeartbeatValidateByPathRequest validates HEARTBEAT.md for the route agent.
type HeartbeatValidateByPathRequest struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Body        string `json:"body"`
}

// HeartbeatPutRequest creates or replaces HEARTBEAT.md through the managed authoring service.
type HeartbeatPutRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	AgentName      string `json:"agent_name"`
	Body           string `json:"body"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// HeartbeatPutByPathRequest creates or replaces HEARTBEAT.md for the route agent.
type HeartbeatPutByPathRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Body           string `json:"body"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// HeartbeatDeleteRequest removes HEARTBEAT.md through the managed authoring service.
type HeartbeatDeleteRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	AgentName      string `json:"agent_name"`
	ExpectedDigest string `json:"expected_digest"`
}

// HeartbeatDeleteByPathRequest removes HEARTBEAT.md for the route agent.
type HeartbeatDeleteByPathRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ExpectedDigest string `json:"expected_digest"`
}

// HeartbeatRollbackRequest restores a prior HEARTBEAT.md revision or snapshot body.
type HeartbeatRollbackRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	AgentName      string `json:"agent_name"`
	RevisionID     string `json:"revision_id,omitempty"`
	TargetDigest   string `json:"target_digest,omitempty"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// HeartbeatRollbackByPathRequest restores a HEARTBEAT.md revision for the route agent.
type HeartbeatRollbackByPathRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	RevisionID     string `json:"revision_id,omitempty"`
	TargetDigest   string `json:"target_digest,omitempty"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// HeartbeatHistoryRequest lists managed HEARTBEAT.md authoring revisions.
type HeartbeatHistoryRequest struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name"`
	Limit       int    `json:"limit,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
}

// HeartbeatStatusRequest composes policy, wake state, and optional session health.
type HeartbeatStatusRequest struct {
	WorkspaceID             string `json:"workspace_id,omitempty"`
	AgentName               string `json:"agent_name"`
	SessionID               string `json:"session_id,omitempty"`
	IncludeSessionHealth    bool   `json:"include_session_health,omitempty"`
	IncludeRecentWakeEvents bool   `json:"include_recent_wake_events,omitempty"`
}

// HeartbeatRevisionPayload is a redacted HEARTBEAT.md authoring history row.
type HeartbeatRevisionPayload struct {
	ID             string                     `json:"id"`
	AgentName      string                     `json:"agent_name"`
	SourcePath     string                     `json:"source_path"`
	Operation      HeartbeatRevisionOperation `json:"operation"`
	PreviousDigest string                     `json:"previous_digest,omitempty"`
	NewDigest      string                     `json:"new_digest,omitempty"`
	NewSnapshotID  string                     `json:"new_snapshot_id,omitempty"`
	Actor          HeartbeatActorPayload      `json:"actor"`
	CreatedAt      time.Time                  `json:"created_at"`
}

// HeartbeatActorPayload records a redacted Heartbeat authoring actor.
type HeartbeatActorPayload struct {
	Kind HeartbeatActorKind `json:"kind"`
	Ref  string             `json:"ref,omitempty"`
}

// HeartbeatHistoryResponse returns bounded HEARTBEAT.md authoring history.
type HeartbeatHistoryResponse struct {
	Revisions  []HeartbeatRevisionPayload `json:"revisions"`
	NextCursor string                     `json:"next_cursor,omitempty"`
}

// HeartbeatMutationResponse returns the post-mutation read model and audit row.
type HeartbeatMutationResponse struct {
	Heartbeat HeartbeatPolicyPayload   `json:"heartbeat"`
	Revision  HeartbeatRevisionPayload `json:"revision"`
}

// SessionHealthPayload is the metadata-only runtime health row for one session.
type SessionHealthPayload struct {
	SessionID           string                           `json:"session_id"`
	WorkspaceID         string                           `json:"workspace_id"`
	AgentName           string                           `json:"agent_name"`
	State               SessionHealthState               `json:"state"`
	Health              SessionHealthStatus              `json:"health"`
	ActivePrompt        bool                             `json:"active_prompt"`
	Attachable          bool                             `json:"attachable"`
	EligibleForWake     bool                             `json:"eligible_for_wake"`
	IneligibilityReason SessionHealthIneligibilityReason `json:"ineligibility_reason,omitempty"`
	LastActivityAt      *time.Time                       `json:"last_activity_at,omitempty"`
	LastPresenceAt      *time.Time                       `json:"last_presence_at,omitempty"`
	LastError           string                           `json:"last_error,omitempty"`
	UpdatedAt           time.Time                        `json:"updated_at"`
}

// SessionHealthResponse wraps one session health read model.
type SessionHealthResponse struct {
	Health SessionHealthPayload `json:"health"`
}

// SessionStatusResponse returns compact session status plus wake eligibility.
type SessionStatusResponse struct {
	SessionID           string                           `json:"session_id"`
	WorkspaceID         string                           `json:"workspace_id"`
	AgentName           string                           `json:"agent_name"`
	State               SessionHealthState               `json:"state"`
	Health              SessionHealthStatus              `json:"health"`
	ActivePrompt        bool                             `json:"active_prompt"`
	Attachable          bool                             `json:"attachable"`
	EligibleForWake     bool                             `json:"eligible_for_wake"`
	IneligibilityReason SessionHealthIneligibilityReason `json:"ineligibility_reason,omitempty"`
	WakeState           *HeartbeatWakeStatePayload       `json:"wake_state,omitempty"`
	UpdatedAt           time.Time                        `json:"updated_at"`
}

// SessionInspectResponse returns detailed health, wake audit, and policy diagnostics.
type SessionInspectResponse struct {
	SessionID    string                             `json:"session_id"`
	Health       SessionHealthPayload               `json:"health"`
	WakeState    *HeartbeatWakeStatePayload         `json:"wake_state,omitempty"`
	WakeEvents   []HeartbeatWakeEventPayload        `json:"wake_events,omitempty"`
	PolicyDigest string                             `json:"policy_digest,omitempty"`
	ConfigDigest string                             `json:"config_digest,omitempty"`
	Diagnostics  []AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
}

// HeartbeatWakeStatePayload is the per-session cooldown/coalescing summary.
type HeartbeatWakeStatePayload struct {
	WorkspaceID      string              `json:"workspace_id,omitempty"`
	AgentName        string              `json:"agent_name,omitempty"`
	SessionID        string              `json:"session_id"`
	PolicySnapshotID string              `json:"policy_snapshot_id,omitempty"`
	LastWakeAt       *time.Time          `json:"last_wake_at,omitempty"`
	NextAllowedAt    *time.Time          `json:"next_allowed_at,omitempty"`
	CoalescedCount   int                 `json:"coalesced_count"`
	LastResult       HeartbeatWakeResult `json:"last_result"`
	LastReason       HeartbeatWakeReason `json:"last_reason,omitempty"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// HeartbeatWakeEventPayload is one retained Heartbeat wake audit row.
type HeartbeatWakeEventPayload struct {
	ID                string              `json:"id"`
	WorkspaceID       string              `json:"workspace_id,omitempty"`
	AgentName         string              `json:"agent_name,omitempty"`
	SessionID         string              `json:"session_id,omitempty"`
	PolicySnapshotID  string              `json:"policy_snapshot_id,omitempty"`
	Source            HeartbeatWakeSource `json:"source"`
	Result            HeartbeatWakeResult `json:"result"`
	Reason            HeartbeatWakeReason `json:"reason"`
	SyntheticPromptID string              `json:"synthetic_prompt_id,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	ExpiresAt         time.Time           `json:"expires_at"`
}

// HeartbeatWakeDecisionPayload reports the auditable result of one wake decision.
type HeartbeatWakeDecisionPayload struct {
	WakeEventID       string                             `json:"wake_event_id,omitempty"`
	Result            HeartbeatWakeResult                `json:"result"`
	Reason            HeartbeatWakeReason                `json:"reason"`
	PolicySnapshotID  string                             `json:"policy_snapshot_id,omitempty"`
	PolicyDigest      string                             `json:"policy_digest,omitempty"`
	ConfigDigest      string                             `json:"config_digest,omitempty"`
	SyntheticPromptID string                             `json:"synthetic_prompt_id,omitempty"`
	Diagnostics       []AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
}

// HeartbeatWakeRequest asks for one manual advisory Heartbeat wake decision.
type HeartbeatWakeRequest struct {
	WorkspaceID    string              `json:"workspace_id,omitempty"`
	AgentName      string              `json:"agent_name"`
	SessionID      string              `json:"session_id"`
	Source         HeartbeatWakeSource `json:"source"`
	DryRun         bool                `json:"dry_run,omitempty"`
	IdempotencyKey string              `json:"idempotency_key,omitempty"`
}

// HeartbeatWakeByPathRequest asks for one wake decision for the route agent.
type HeartbeatWakeByPathRequest struct {
	WorkspaceID    string              `json:"workspace_id,omitempty"`
	SessionID      string              `json:"session_id"`
	Source         HeartbeatWakeSource `json:"source"`
	DryRun         bool                `json:"dry_run,omitempty"`
	IdempotencyKey string              `json:"idempotency_key,omitempty"`
}

// HeartbeatWakeResponse wraps one advisory wake decision.
type HeartbeatWakeResponse struct {
	Decision HeartbeatWakeDecisionPayload `json:"decision"`
}

// HeartbeatStatusResponse composes policy status, wake state, recent wake audit, and optional health.
type HeartbeatStatusResponse struct {
	AgentName        string                             `json:"agent_name"`
	SourcePath       string                             `json:"source_path,omitempty"`
	Enabled          bool                               `json:"enabled"`
	Present          bool                               `json:"present"`
	Active           bool                               `json:"active"`
	Valid            bool                               `json:"valid"`
	ValidationStatus AuthoredValidationStatus           `json:"validation_status"`
	Digest           string                             `json:"digest,omitempty"`
	ConfigDigest     string                             `json:"config_digest,omitempty"`
	SnapshotID       string                             `json:"snapshot_id,omitempty"`
	Summary          string                             `json:"summary,omitempty"`
	Preferences      HeartbeatPreferencesPayload        `json:"preferences"`
	Diagnostics      []AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
	WakeState        *HeartbeatWakeStatePayload         `json:"wake_state,omitempty"`
	WakeEvents       []HeartbeatWakeEventPayload        `json:"wake_events,omitempty"`
	SessionHealth    *SessionHealthPayload              `json:"session_health,omitempty"`
	RevisionCursor   string                             `json:"revision_cursor,omitempty"`
}

// SessionPayloadHealth attaches optional health to session list/read responses when requested.
type SessionPayloadHealth = SessionHealthPayload
