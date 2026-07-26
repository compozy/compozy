package task

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

// CreateTask captures the mutable inputs accepted when creating a new task.
type CreateTask struct {
	ID                   string                 `json:"id,omitempty"`
	Identifier           string                 `json:"identifier,omitempty"`
	Scope                Scope                  `json:"scope"`
	WorkspaceID          string                 `json:"workspace_id,omitempty"`
	ParentTaskID         string                 `json:"parent_task_id,omitempty"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description,omitempty"`
	Priority             Priority               `json:"priority,omitempty"`
	MaxAttempts          *int                   `json:"max_attempts,omitempty"`
	Draft                bool                   `json:"draft,omitempty"`
	AutoEnqueueOnReady   bool                   `json:"auto_enqueue_on_ready,omitempty"`
	ApprovalPolicy       ApprovalPolicy         `json:"approval_policy,omitempty"`
	Owner                *Ownership             `json:"owner,omitempty"`
	WakeCreator          *bool                  `json:"wake_creator,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
}

// Patch captures the mutable task fields accepted by update operations.
type Patch struct {
	Title                *string                `json:"title,omitempty"`
	Description          *string                `json:"description,omitempty"`
	Priority             *Priority              `json:"priority,omitempty"`
	MaxAttempts          *int                   `json:"max_attempts,omitempty"`
	AutoEnqueueOnReady   *bool                  `json:"auto_enqueue_on_ready,omitempty"`
	ApprovalPolicy       *ApprovalPolicy        `json:"approval_policy,omitempty"`
	Metadata             *json.RawMessage       `json:"metadata,omitempty"`
	Owner                *Ownership             `json:"owner,omitempty"`
	ClearOwner           bool                   `json:"clear_owner,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

// CancelTask captures the task-level cancellation request payload.
type CancelTask struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ExecutionAction identifies the operator action that crossed the
// create-versus-execute lifecycle boundary.
type ExecutionAction string

const (
	// ExecutionActionStart records an explicit operator start request.
	ExecutionActionStart ExecutionAction = "start"
	// ExecutionActionPublish records a draft publish request that also starts execution.
	ExecutionActionPublish ExecutionAction = "publish"
	// ExecutionActionApproval records an approval request that also starts execution.
	ExecutionActionApproval ExecutionAction = "approval"
)

// ExecutionRequest captures the mutable inputs accepted when an operator
// publish, start, or approval action enqueues executable work.
type ExecutionRequest struct {
	IdempotencyKey       string                 `json:"idempotency_key,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
}

// Execution captures the task and run created or resolved at the explicit
// execution boundary.
type Execution struct {
	Task        Task            `json:"task"`
	Run         Run             `json:"run"`
	Action      ExecutionAction `json:"action"`
	ExistingRun bool            `json:"existing_run,omitempty"`
}

// AddDependency captures one dependency-edge creation request.
type AddDependency struct {
	TaskID          string         `json:"task_id"`
	DependsOnTaskID string         `json:"depends_on_task_id"`
	Kind            DependencyKind `json:"kind"`
}

// EnqueueRun captures the mutable inputs accepted when queuing a task run.
type EnqueueRun struct {
	TaskID                     string                 `json:"task_id"`
	RunKind                    RunKind                `json:"run_kind,omitempty"`
	LoopRunID                  string                 `json:"loop_run_id,omitempty"`
	IdempotencyKey             string                 `json:"idempotency_key,omitempty"`
	NetworkParticipation       *participation.Request `json:"network_participation,omitempty"`
	NetworkParticipationSource participation.Source   `json:"-"`
	DesignationGroupID         string                 `json:"designation_group_id,omitempty"`
	Metadata                   json.RawMessage        `json:"metadata,omitempty"`
}

// StartRun captures one run-start request.
type StartRun struct {
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	ClaimToken     string `json:"claim_token,omitempty"`
}

// CancelRun captures one run-cancellation request.
type CancelRun struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RunFailure captures the durable failure payload returned by a failed run.
type RunFailure struct {
	Error    string          `json:"error"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ForceRecoveryOptions controls operator/agent force-operation policy.
type ForceRecoveryOptions struct {
	AllowAgentForce    bool `json:"allow_agent_force"`
	RateLimitPerMinute int  `json:"rate_limit_per_minute,omitempty"`
}

// ForceReleaseRun captures one operator/agent force release request.
type ForceReleaseRun struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ForceFailRun captures one operator/agent forced failure request.
type ForceFailRun struct {
	Reason   string          `json:"reason"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RetryRunRequest captures one operator/agent retry request.
type RetryRunRequest struct {
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RetryRunResult records the source terminal run and the newly queued retry.
type RetryRunResult struct {
	PreviousRun Run `json:"previous_run"`
	Run         Run `json:"run"`
}

// BulkForceRunRequest captures a bounded release/fail batch.
type BulkForceRunRequest struct {
	RunIDs   []string        `json:"run_ids"`
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// BulkForceRunItem records one per-run bulk recovery outcome.
type BulkForceRunItem struct {
	RunID string `json:"run_id"`
	OK    bool   `json:"ok"`
	Run   *Run   `json:"run,omitempty"`
	Err   error  `json:"-"`
}

// BulkForceRunResult records bounded per-row outcomes.
type BulkForceRunResult struct {
	Items []BulkForceRunItem `json:"items"`
}

// PauseTaskRequest captures one per-task pause request.
type PauseTaskRequest struct {
	Reason   string          `json:"reason"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ResumeTaskRequest captures one per-task resume request.
type ResumeTaskRequest struct {
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// PauseMutation captures one persisted per-task pause write.
type PauseMutation struct {
	TaskID   string    `json:"task_id"`
	Actor    string    `json:"actor"`
	Reason   string    `json:"reason"`
	PausedAt time.Time `json:"paused_at"`
}

// ResumeMutation captures one persisted per-task resume write.
type ResumeMutation struct {
	TaskID    string    `json:"task_id"`
	ResumedAt time.Time `json:"resumed_at"`
}

// PauseState reports direct and inherited pause state for one task.
type PauseState struct {
	TaskID          string    `json:"task_id"`
	Paused          bool      `json:"paused"`
	PausedBy        string    `json:"paused_by,omitempty"`
	PausedAt        time.Time `json:"paused_at,omitzero"`
	PausedReason    string    `json:"paused_reason,omitempty"`
	EffectivePaused bool      `json:"effective_paused"`
	PausedByTaskID  string    `json:"paused_by_task_id,omitempty"`
}

// SchedulerPauseState records the singleton scheduler-wide pause state.
type SchedulerPauseState struct {
	Paused    bool      `json:"paused"`
	PausedBy  string    `json:"paused_by,omitempty"`
	PausedAt  time.Time `json:"paused_at,omitzero"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// SchedulerStatus reports scheduler-wide pause state and live backlog counts.
type SchedulerStatus struct {
	Paused                 bool      `json:"paused"`
	PausedBy               string    `json:"paused_by,omitempty"`
	PausedAt               time.Time `json:"paused_at,omitzero"`
	PausedReason           string    `json:"paused_reason,omitempty"`
	ActiveClaimCount       int       `json:"active_claim_count"`
	QueuedRunCount         int       `json:"queued_run_count"`
	PausedTaskCount        int       `json:"paused_task_count"`
	StarvedRunCount        int       `json:"starved_run_count"`
	NeedsAttentionRunCount int       `json:"needs_attention_run_count"`
	AsOf                   time.Time `json:"as_of"`
}

// SchedulerPauseRequest captures one scheduler-wide pause request.
type SchedulerPauseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// SchedulerPauseMutation captures one scheduler-wide pause-state write.
type SchedulerPauseMutation struct {
	Paused    bool      `json:"paused"`
	Actor     string    `json:"actor,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SchedulerResumeRequest captures one scheduler-wide resume request.
type SchedulerResumeRequest struct {
	Reason string `json:"reason,omitempty"`
}

// SchedulerDrainRequest captures one drain invocation.
type SchedulerDrainRequest struct {
	Reason  string        `json:"reason,omitempty"`
	Timeout time.Duration `json:"timeout,omitempty"`
}

// SchedulerDrainResult reports the final state observed by one drain invocation.
type SchedulerDrainResult struct {
	Status          SchedulerStatus `json:"status"`
	Completed       bool            `json:"completed"`
	TimedOut        bool            `json:"timed_out,omitempty"`
	RemainingClaims int             `json:"remaining_claims"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     time.Time       `json:"completed_at"`
}

// SchedulerBacklogQuery captures read filters for queued scheduler backlog.
type SchedulerBacklogQuery struct {
	Limit         int    `json:"limit,omitempty"`
	Scope         Scope  `json:"scope,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	IncludePaused bool   `json:"include_paused,omitempty"`
}

// SchedulerBacklogRun joins one queued run with the task that owns it.
type SchedulerBacklogRun struct {
	Task            Task   `json:"task"`
	Run             Run    `json:"run"`
	EffectivePaused bool   `json:"effective_paused"`
	PausedByTaskID  string `json:"paused_by_task_id,omitempty"`
}

// SchedulerBacklog reports queued scheduler backlog rows and the unbounded total.
type SchedulerBacklog struct {
	Runs  []SchedulerBacklogRun `json:"runs"`
	Total int                   `json:"total"`
}

// ForceRunMutationResult records the before/after state for one force mutation.
type ForceRunMutationResult struct {
	Previous Run `json:"previous"`
	Run      Run `json:"run"`
}

// ForceReleaseRunMutation captures one transactional force-release write.
type ForceReleaseRunMutation struct {
	RunID string    `json:"run_id"`
	Now   time.Time `json:"now"`
}

// ForceFailRunMutation captures one transactional force-fail write.
type ForceFailRunMutation struct {
	RunID  string    `json:"run_id"`
	Reason string    `json:"reason"`
	Now    time.Time `json:"now"`
}

// RetryRunMutation captures one transactional retry write.
type RetryRunMutation struct {
	SourceRunID string          `json:"source_run_id"`
	NewRunID    string          `json:"new_run_id"`
	Origin      Origin          `json:"origin"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	QueuedAt    time.Time       `json:"queued_at"`
}

// RecoverRunRequest captures one operator/agent recovery request for a needs_attention run.
type RecoverRunRequest struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RecoverRunMutation captures one transactional recovery write (terminalize-then-requeue).
type RecoverRunMutation struct {
	SourceRunID string          `json:"source_run_id"`
	NewRunID    string          `json:"new_run_id"`
	Origin      Origin          `json:"origin"`
	Reason      string          `json:"reason,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	QueuedAt    time.Time       `json:"queued_at"`
}
