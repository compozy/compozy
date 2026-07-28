package hooks

import "time"

// AgentPreStartPayload is delivered before an agent process starts.
type AgentPreStartPayload struct {
	PayloadBase
	SessionContext
	Command  string   `json:"command,omitempty"`
	Args     []string `json:"args,omitempty"`
	Cwd      string   `json:"cwd,omitempty"`
	Provider string   `json:"provider,omitempty"`
	Model    string   `json:"model,omitempty"`
}

// AgentLifecyclePayload is shared by spawned, crashed, and stopped hooks.
type AgentLifecyclePayload struct {
	PayloadBase
	SessionContext
	Command  string   `json:"command,omitempty"`
	Args     []string `json:"args,omitempty"`
	Cwd      string   `json:"cwd,omitempty"`
	PID      int      `json:"pid,omitempty"`
	Provider string   `json:"provider,omitempty"`
	Model    string   `json:"model,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// AgentSpawnedPayload is delivered after an agent process starts.
type AgentSpawnedPayload = AgentLifecyclePayload

// AgentCrashedPayload is delivered when an agent crashes.
type AgentCrashedPayload = AgentLifecyclePayload

// AgentStoppedPayload is delivered after an agent stops.
type AgentStoppedPayload = AgentLifecyclePayload

// AgentStartPatch mutates or denies a pre-start operation.
type AgentStartPatch struct {
	ControlPatch
	Command *string  `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Cwd     *string  `json:"cwd,omitempty"`
}

// AgentLifecyclePatch captures optional labels for observation events.
type AgentLifecyclePatch struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// AgentSpawnedPatch is the spawned patch surface.
type AgentSpawnedPatch = AgentLifecyclePatch

// AgentCrashedPatch is the crashed patch surface.
type AgentCrashedPatch = AgentLifecyclePatch

// AgentStoppedPatch is the stopped patch surface.
type AgentStoppedPatch = AgentLifecyclePatch

// AuthoredContextProvenance carries redacted Soul/Heartbeat source identity.
type AuthoredContextProvenance struct {
	WorkspaceID      string `json:"workspace_id,omitempty"`
	AgentName        string `json:"agent_name,omitempty"`
	SourcePath       string `json:"source_path,omitempty"`
	SnapshotID       string `json:"snapshot_id,omitempty"`
	Digest           string `json:"digest,omitempty"`
	ConfigDigest     string `json:"config_digest,omitempty"`
	ValidationStatus string `json:"validation_status,omitempty"`
	Valid            bool   `json:"valid"`
	Active           bool   `json:"active"`
	Reason           string `json:"reason,omitempty"`
}

// AuthoredMutationProvenance records who caused a managed authored-context mutation.
type AuthoredMutationProvenance struct {
	ActorKind  string `json:"actor_kind,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	OriginKind string `json:"origin_kind,omitempty"`
	OriginRef  string `json:"origin_ref,omitempty"`
}

// AgentSoulSnapshotResolvedPayload is delivered after Soul snapshot/read-model resolution.
type AgentSoulSnapshotResolvedPayload struct {
	PayloadBase
	AuthoredContextProvenance
}

// AgentSoulMutationAfterPayload is delivered after a managed SOUL.md mutation commits.
type AgentSoulMutationAfterPayload struct {
	PayloadBase
	AuthoredContextProvenance
	AuthoredMutationProvenance
	RevisionID     string `json:"revision_id,omitempty"`
	Action         string `json:"action,omitempty"`
	PreviousDigest string `json:"previous_digest,omitempty"`
	NewDigest      string `json:"new_digest,omitempty"`
}

// AgentHeartbeatPolicyResolvedPayload is delivered after Heartbeat policy/status resolution.
type AgentHeartbeatPolicyResolvedPayload struct {
	PayloadBase
	AuthoredContextProvenance
	Summary string `json:"summary,omitempty"`
}

// AgentHeartbeatWakeBeforePayload is delivered before a managed Heartbeat wake decision.
type AgentHeartbeatWakeBeforePayload struct {
	PayloadBase
	SessionContext
	PolicySnapshotID string `json:"policy_snapshot_id,omitempty"`
	PolicyDigest     string `json:"policy_digest,omitempty"`
	ConfigDigest     string `json:"config_digest,omitempty"`
	Source           string `json:"source,omitempty"`
	DryRun           bool   `json:"dry_run,omitempty"`
}

// AgentHeartbeatWakeAfterPayload is delivered after a managed Heartbeat wake decision.
type AgentHeartbeatWakeAfterPayload struct {
	PayloadBase
	SessionContext
	WakeEventID       string `json:"wake_event_id,omitempty"`
	Result            string `json:"result,omitempty"`
	Reason            string `json:"reason,omitempty"`
	PolicySnapshotID  string `json:"policy_snapshot_id,omitempty"`
	PolicyDigest      string `json:"policy_digest,omitempty"`
	ConfigDigest      string `json:"config_digest,omitempty"`
	SyntheticPromptID string `json:"synthetic_prompt_id,omitempty"`
	Source            string `json:"source,omitempty"`
}

// SessionHealthUpdateAfterPayload is delivered after metadata-only session health changes.
type SessionHealthUpdateAfterPayload struct {
	PayloadBase
	SessionContext
	Health              string    `json:"health,omitempty"`
	ActivePrompt        bool      `json:"active_prompt,omitempty"`
	Attachable          bool      `json:"attachable,omitempty"`
	EligibleForWake     bool      `json:"eligible_for_wake,omitempty"`
	IneligibilityReason string    `json:"ineligibility_reason,omitempty"`
	LastActivityAt      time.Time `json:"last_activity_at"`
	LastPresenceAt      time.Time `json:"last_presence_at"`
	LastError           string    `json:"last_error,omitempty"`
}
