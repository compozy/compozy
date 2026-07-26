package task

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

type createdTaskPayload struct {
	Scope        Scope      `json:"scope"`
	WorkspaceID  string     `json:"workspace_id,omitempty"`
	ParentTaskID string     `json:"parent_task_id,omitempty"`
	Status       Status     `json:"status"`
	Owner        *Ownership `json:"owner,omitempty"`
}

type updatedTaskPayload struct {
	ChangedFields []string `json:"changed_fields"`
	Status        Status   `json:"status"`
}

type publishedTaskPayload struct {
	PreviousStatus Status        `json:"previous_status"`
	Status         Status        `json:"status"`
	ApprovalState  ApprovalState `json:"approval_state"`
}

type approvalDecisionTaskPayload struct {
	PreviousApprovalState ApprovalState `json:"previous_approval_state"`
	ApprovalState         ApprovalState `json:"approval_state"`
	Status                Status        `json:"status"`
}

type childCreatedTaskPayload struct {
	ChildTaskID      string `json:"child_task_id"`
	ChildScope       Scope  `json:"child_scope"`
	ChildWorkspaceID string `json:"child_workspace_id,omitempty"`
}

type dependencyTaskPayload struct {
	DependsOnTaskID string         `json:"depends_on_task_id"`
	Kind            DependencyKind `json:"kind"`
	Status          Status         `json:"status"`
}

type cancelledTaskPayload struct {
	Reason               string          `json:"reason,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	Status               Status          `json:"status"`
	PropagatedFromTaskID string          `json:"propagated_from_task_id,omitempty"`
	CancelledRunIDs      []string        `json:"canceled_run_ids,omitempty"`
}

type runEnqueuedPayload struct {
	Attempt        int       `json:"attempt"`
	Status         RunStatus `json:"status"`
	TaskStatus     Status    `json:"task_status"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
}

type runClaimedPayload struct {
	Status         RunStatus     `json:"status"`
	TaskStatus     Status        `json:"task_status"`
	ClaimedBy      ActorIdentity `json:"claimed_by"`
	ClaimTokenHash string        `json:"claim_token_hash,omitempty"`
	LeaseUntil     time.Time     `json:"lease_until"`
}

type runTransitionPayload struct {
	Status     RunStatus `json:"status"`
	TaskStatus Status    `json:"task_status"`
	SessionID  string    `json:"session_id,omitempty"`
}

type completedRunPayload struct {
	Status         RunStatus       `json:"status"`
	TaskStatus     Status          `json:"task_status"`
	Result         json.RawMessage `json:"result,omitempty"`
	ClaimTokenHash string          `json:"claim_token_hash,omitempty"`
}

type completionHallucinationBlockedPayload struct {
	Status         RunStatus `json:"status"`
	SessionID      string    `json:"session_id,omitempty"`
	ClaimedTaskIDs []string  `json:"claimed_task_ids,omitempty"`
	InvalidTaskIDs []string  `json:"invalid_task_ids,omitempty"`
	ClaimTokenHash string    `json:"claim_token_hash,omitempty"`
}

type completionHallucinationSuspectedPayload struct {
	Status           RunStatus `json:"status"`
	SuspectedTaskIDs []string  `json:"suspected_task_ids,omitempty"`
	ClaimTokenHash   string    `json:"claim_token_hash,omitempty"`
}

type failedRunPayload struct {
	Status         RunStatus       `json:"status"`
	TaskStatus     Status          `json:"task_status"`
	Error          string          `json:"error"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	ClaimTokenHash string          `json:"claim_token_hash,omitempty"`
}

type cancelledRunPayload struct {
	Status                   RunStatus       `json:"status"`
	TaskStatus               Status          `json:"task_status,omitempty"`
	Reason                   string          `json:"reason,omitempty"`
	Metadata                 json.RawMessage `json:"metadata,omitempty"`
	SessionID                string          `json:"session_id,omitempty"`
	PropagatedFromTaskID     string          `json:"propagated_from_task_id,omitempty"`
	CooperativeStopRequested bool            `json:"cooperative_stop_requested,omitempty"`
}

type forceStoppedRunPayload struct {
	SessionID            string `json:"session_id"`
	GraceTimeoutMillis   int64  `json:"grace_timeout_ms"`
	PropagatedFromTaskID string `json:"propagated_from_task_id,omitempty"`
}

type recoveredRunPayload struct {
	Action         RunBootRecoveryAction `json:"action"`
	PreviousStatus RunStatus             `json:"previous_status"`
	Status         RunStatus             `json:"status"`
	TaskStatus     Status                `json:"task_status"`
	Reason         string                `json:"reason,omitempty"`
	SessionID      string                `json:"session_id,omitempty"`
	SessionState   string                `json:"session_state,omitempty"`
	Classification string                `json:"classification,omitempty"`
	Detail         string                `json:"detail,omitempty"`
}

type leaseExtendedPayload struct {
	Status         RunStatus `json:"status"`
	TaskStatus     Status    `json:"task_status"`
	LeaseUntil     time.Time `json:"lease_until"`
	HeartbeatAt    time.Time `json:"heartbeat_at"`
	ClaimTokenHash string    `json:"claim_token_hash,omitempty"`
}

type releasedRunPayload struct {
	Manual                          bool       `json:"manual,omitempty"`
	ActorKind                       ActorKind  `json:"actor_kind,omitempty"`
	ActorID                         string     `json:"actor_id,omitempty"`
	PreviousStatus                  RunStatus  `json:"previous_status"`
	Status                          RunStatus  `json:"status"`
	TaskStatus                      Status     `json:"task_status"`
	Reason                          string     `json:"reason,omitempty"`
	SessionID                       string     `json:"session_id,omitempty"`
	PreviousSessionID               string     `json:"previous_session_id,omitempty"`
	PreviousClaimTokenHashTruncated string     `json:"previous_claim_token_hash_truncated,omitempty"`
	PreviousLeaseUntil              *time.Time `json:"previous_lease_until,omitempty"`
	QueueGeneration                 int64      `json:"queue_generation,omitempty"`
	CanceledQueuedInputs            int        `json:"canceled_queued_inputs,omitempty"`
}

// ExpiredLeaseEventPayload is the canonical audit payload for an expired task-run lease.
type ExpiredLeaseEventPayload struct {
	PreviousStatus               RunStatus           `json:"previous_status"`
	Status                       RunStatus           `json:"status"`
	TaskStatus                   Status              `json:"task_status"`
	Reason                       string              `json:"reason,omitempty"`
	SessionID                    string              `json:"session_id,omitempty"`
	LeaseUntil                   time.Time           `json:"lease_until"`
	PreviousTokenHash            string              `json:"previous_claim_token_hash,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation"`
}
