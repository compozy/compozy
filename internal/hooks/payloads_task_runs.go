package hooks

import (
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

// TaskRunClaimCriteria carries the mutable claim criteria exposed to task-run pre-claim hooks.
type TaskRunClaimCriteria struct {
	RunID                string   `json:"run_id,omitempty"`
	RunKind              string   `json:"run_kind,omitempty"`
	WorkspaceID          string   `json:"workspace_id,omitempty"`
	TargetSessionID      string   `json:"target_session_id,omitempty"`
	ClaimerSessionID     string   `json:"claimer_session_id,omitempty"`
	AgentName            string   `json:"agent_name,omitempty"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	PriorityMin          int      `json:"priority_min,omitempty"`
}

// TaskRunContext carries task-run identifiers shared across task-run hooks.
type TaskRunContext struct {
	TaskID                       string              `json:"task_id,omitempty"`
	RunID                        string              `json:"run_id,omitempty"`
	RunKind                      *string             `json:"run_kind,omitempty"`
	WakeID                       string              `json:"wake_id,omitempty"`
	OwnerKey                     string              `json:"owner_key,omitempty"`
	TargetSessionID              string              `json:"target_session_id,omitempty"`
	LoopRunID                    string              `json:"loop_run_id,omitempty"`
	WorkspaceID                  string              `json:"workspace_id,omitempty"`
	WorkflowID                   string              `json:"workflow_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation"`
	AgentName                    string              `json:"agent_name,omitempty"`
	SessionID                    string              `json:"session_id,omitempty"`
	ActorKind                    string              `json:"actor_kind,omitempty"`
	ActorID                      string              `json:"actor_id,omitempty"`
	OriginKind                   string              `json:"origin_kind,omitempty"`
	OriginRef                    string              `json:"origin_ref,omitempty"`
	TaskStatus                   string              `json:"task_status,omitempty"`
	RunStatus                    string              `json:"run_status,omitempty"`
	SoulSnapshotID               string              `json:"soul_snapshot_id,omitempty"`
	SoulDigest                   string              `json:"soul_digest,omitempty"`
	Attempt                      int                 `json:"attempt,omitempty"`
	LeaseUntil                   time.Time           `json:"lease_until"`
	ReleaseReason                string              `json:"release_reason,omitempty"`
	Error                        string              `json:"error,omitempty"`
}

// TaskRunEnqueuedPayload is delivered after a task run is enqueued and its audit event is committed.
type TaskRunEnqueuedPayload struct {
	PayloadBase
	TaskRunContext
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// TaskRunPreClaimPayload is delivered before a task run claim commits.
type TaskRunPreClaimPayload struct {
	PayloadBase
	*TaskRunContext
	Criteria   TaskRunClaimCriteria `json:"criteria"`
	Denied     bool                 `json:"denied,omitempty"`
	DenyReason string               `json:"deny_reason,omitempty"`
}

// TaskRunPostClaimPayload is delivered after a task run claim and audit event commit.
type TaskRunPostClaimPayload struct {
	PayloadBase
	TaskRunContext
	ClaimedAt time.Time `json:"claimed_at"`
}

// TaskRunLeasePayload is shared by committed task-run lease lifecycle hooks.
type TaskRunLeasePayload struct {
	PayloadBase
	TaskRunContext
	PreviousRunStatus string `json:"previous_run_status,omitempty"`
	PreviousSessionID string `json:"previous_session_id,omitempty"`
	RecoveryAction    string `json:"recovery_action,omitempty"`
	RecoveryReason    string `json:"recovery_reason,omitempty"`
}

// TaskRunLeaseExtendedPayload is delivered after a task-run lease is extended.
type TaskRunLeaseExtendedPayload = TaskRunLeasePayload

// TaskRunLeaseExpiredPayload is delivered after a task-run lease expires.
type TaskRunLeaseExpiredPayload = TaskRunLeasePayload

// TaskRunLeaseRecoveredPayload is delivered after lease recovery commits.
type TaskRunLeaseRecoveredPayload = TaskRunLeasePayload

// TaskRunReleasedPayload is delivered after a task run lease is released.
type TaskRunReleasedPayload = TaskRunLeasePayload

// TaskRunCompletedPayload is delivered after a token-fenced task run completion.
type TaskRunCompletedPayload = TaskRunLeasePayload

// TaskRunFailedPayload is delivered after a token-fenced task run failure.
type TaskRunFailedPayload = TaskRunLeasePayload

// TaskRunPreClaimPatch denies or narrows task-run claim criteria.
type TaskRunPreClaimPatch struct {
	ControlPatch
	AddRequiredCapabilities []string `json:"add_required_capabilities,omitempty"`
	PriorityMin             *int     `json:"priority_min,omitempty"`
}

// TaskRunObservationPatch is the observation patch surface for committed task-run hooks.
type TaskRunObservationPatch = AutonomyObservationPatch
