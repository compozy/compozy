package task

import (
	"encoding/json"
	"time"
)

// Dependency is the durable edge record connecting one task to a blocking dependency.
type Dependency struct {
	TaskID          string         `json:"task_id"`
	DependsOnTaskID string         `json:"depends_on_task_id"`
	Kind            DependencyKind `json:"kind"`
	CreatedAt       time.Time      `json:"created_at"`
}

// RunReviewLineage captures review-gate fields attached to a task run.
type RunReviewLineage struct {
	Required           bool            `json:"required,omitempty"`
	RequestRound       int             `json:"request_round,omitempty"`
	PolicySnapshot     ReviewPolicy    `json:"policy_snapshot,omitempty"`
	RequestID          string          `json:"request_id,omitempty"`
	ParentRunID        string          `json:"parent_run_id,omitempty"`
	ReviewID           string          `json:"review_id,omitempty"`
	ReviewRound        int             `json:"review_round,omitempty"`
	ContinuationReason string          `json:"continuation_reason,omitempty"`
	MissingWork        json.RawMessage `json:"missing_work,omitempty"`
	NextRoundGuidance  string          `json:"next_round_guidance,omitempty"`
}

// Run is the durable execution record for one task attempt.
type Run struct {
	ID             string         `json:"id"`
	TaskID         string         `json:"task_id"`
	WorkspaceID    string         `json:"workspace_id,omitempty"`
	Attempt        int32          `json:"attempt"`
	RecoveryCount  int32          `json:"recovery_count"`
	RunKind        RunKind        `json:"run_kind,omitempty"`
	Status         RunStatus      `json:"status"`
	LoopRunID      string         `json:"loop_run_id,omitempty"`
	PreviousRunID  string         `json:"previous_run_id,omitempty"`
	FailureKind    string         `json:"failure_kind,omitempty"`
	ClaimedBy      *ActorIdentity `json:"claimed_by,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	Origin         Origin         `json:"origin"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	*RunNetworkState
	DesignationGroupID    string            `json:"designation_group_id,omitempty"`
	ClaimTokenHash        string            `json:"claim_token_hash,omitempty"`
	LeaseUntil            time.Time         `json:"lease_until"`
	HeartbeatAt           time.Time         `json:"heartbeat_at"`
	RequiredCapabilities  []string          `json:"required_capabilities,omitempty"`
	PreferredCapabilities []string          `json:"preferred_capabilities,omitempty"`
	Review                *RunReviewLineage `json:"review,omitempty"`
	Metadata              json.RawMessage   `json:"metadata,omitempty"`
	QueuedAt              time.Time         `json:"queued_at"`
	ClaimedAt             time.Time         `json:"claimed_at"`
	StartedAt             time.Time         `json:"started_at"`
	EndedAt               time.Time         `json:"ended_at"`
	TokensUsed            int64             `json:"tokens_used,omitempty"`
	Error                 string            `json:"error,omitempty"`
	Result                json.RawMessage   `json:"result,omitempty"`
}

// Event is the immutable audit record emitted for task-domain actions.
type Event struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"task_id"`
	RunID     string          `json:"run_id,omitempty"`
	EventType string          `json:"event_type"`
	Actor     ActorIdentity   `json:"actor"`
	Origin    Origin          `json:"origin"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// TriageState is the durable actor-scoped inbox and triage state for one task.
type TriageState struct {
	TaskID             string        `json:"task_id"`
	Actor              ActorIdentity `json:"actor"`
	Read               bool          `json:"read"`
	Archived           bool          `json:"archived"`
	Dismissed          bool          `json:"dismissed"`
	LastSeenActivityAt time.Time     `json:"last_seen_activity_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// Summary is the lightweight read model returned from list-oriented task queries.
type Summary struct {
	ID              string                `json:"id"`
	Identifier      string                `json:"identifier,omitempty"`
	Scope           Scope                 `json:"scope"`
	WorkspaceID     string                `json:"workspace_id,omitempty"`
	ParentTaskID    string                `json:"parent_task_id,omitempty"`
	Title           string                `json:"title"`
	Priority        Priority              `json:"priority,omitempty"`
	Status          Status                `json:"status"`
	BlockedReasons  *[]BlockedReason      `json:"blocked_reasons,omitempty"`
	NeedsAttention  *NeedsAttention       `json:"needs_attention,omitempty"`
	ApprovalPolicy  ApprovalPolicy        `json:"approval_policy,omitempty"`
	ApprovalState   ApprovalState         `json:"approval_state,omitempty"`
	CurrentRunID    string                `json:"current_run_id,omitempty"`
	PausedBy        string                `json:"paused_by,omitempty"`
	PausedReason    string                `json:"paused_reason,omitempty"`
	PausedByTaskID  string                `json:"paused_by_task_id,omitempty"`
	CreatedBy       ActorIdentity         `json:"created_by"`
	Origin          Origin                `json:"origin"`
	Owner           *Ownership            `json:"owner,omitempty"`
	Dependencies    []DependencyReference `json:"dependencies,omitempty"`
	ActiveRun       *RunSummary           `json:"active_run,omitempty"`
	MaxAttempts     int                   `json:"max_attempts,omitempty"`
	LatestEventSeq  int64                 `json:"latest_event_seq"`
	ChildCount      int32                 `json:"child_count,omitempty"`
	DependencyCount int32                 `json:"dependency_count,omitempty"`
	// Bool fields are clustered to keep Summary within the 512-byte gocritic copy threshold.
	AutoEnqueueOnReady bool      `json:"auto_enqueue_on_ready,omitempty"`
	Draft              bool      `json:"draft"`
	Paused             bool      `json:"paused,omitempty"`
	EffectivePaused    bool      `json:"effective_paused,omitempty"`
	WakeCreator        bool      `json:"wake_creator"`
	PausedAt           time.Time `json:"paused_at,omitzero"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	ClosedAt           time.Time `json:"closed_at"`
	LastActivityAt     time.Time `json:"last_activity_at"`
}
