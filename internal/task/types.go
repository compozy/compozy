package task

import "encoding/json"

// Scope identifies whether a task is daemon-global or workspace-scoped.
type Scope string

const (
	// ScopeGlobal identifies a daemon-wide task with no workspace binding.
	ScopeGlobal Scope = "global"
	// ScopeWorkspace identifies a task bound to one workspace.
	ScopeWorkspace Scope = "workspace"
)

// Status identifies the canonical lifecycle state of a task.
type Status string

const (
	// TaskStatusDraft reports a saved draft that is not yet runnable.
	TaskStatusDraft Status = "draft"
	// TaskStatusPending reports a task that exists but has not yet been reconciled into ready work.
	TaskStatusPending Status = "pending"
	// TaskStatusBlocked reports a task with unresolved dependencies.
	TaskStatusBlocked Status = "blocked"
	// TaskStatusNeedsAttention reports a task escalated out of normal claim flow for recovery.
	TaskStatusNeedsAttention Status = "needs_attention"
	// TaskStatusReady reports a task that may execute because dependencies are satisfied.
	TaskStatusReady Status = "ready"
	// TaskStatusInProgress reports a task with an active starting or running run.
	TaskStatusInProgress Status = "in_progress"
	// TaskStatusCompleted reports a task that finished successfully.
	TaskStatusCompleted Status = "completed"
	// TaskStatusFailed reports a task that ended unsuccessfully.
	TaskStatusFailed Status = "failed"
	// TaskStatusCanceled reports a task that was canceled before successful completion.
	TaskStatusCanceled Status = "canceled"
)

// Priority identifies the operator-facing urgency assigned to one task.
type Priority string

const (
	// PriorityLow identifies the lowest urgency.
	PriorityLow Priority = "low"
	// PriorityMedium identifies the default urgency.
	PriorityMedium Priority = "medium"
	// PriorityHigh identifies elevated urgency.
	PriorityHigh Priority = "high"
	// PriorityUrgent identifies the highest urgency.
	PriorityUrgent Priority = "urgent"
	// DefaultPriority is the canonical priority used when callers omit the field.
	DefaultPriority Priority = PriorityMedium
)

// ApprovalPolicy identifies whether a task requires an explicit approval step.
type ApprovalPolicy string

const (
	// ApprovalPolicyNone identifies tasks that do not require approval.
	ApprovalPolicyNone ApprovalPolicy = "none"
	// ApprovalPolicyManual identifies tasks that require an explicit approve or reject action.
	ApprovalPolicyManual ApprovalPolicy = "manual"
	// DefaultApprovalPolicy is the canonical policy used when callers omit approval requirements.
	DefaultApprovalPolicy ApprovalPolicy = ApprovalPolicyNone
)

// ApprovalState identifies the current approval outcome for one task.
type ApprovalState string

const (
	// ApprovalStateNotRequired identifies tasks whose policy does not require approval.
	ApprovalStateNotRequired ApprovalState = "not_required"
	// ApprovalStatePending identifies tasks waiting for approval.
	ApprovalStatePending ApprovalState = "pending"
	// ApprovalStateApproved identifies tasks that were approved.
	ApprovalStateApproved ApprovalState = "approved"
	// ApprovalStateRejected identifies tasks that were rejected.
	ApprovalStateRejected ApprovalState = "rejected"
)

// RunStatus identifies the canonical lifecycle state of a task run.
type RunStatus uint8

const (
	taskRunStatusQueuedString         = "queued"
	taskRunStatusClaimedString        = "claimed"
	taskRunStatusStartingString       = "starting"
	taskRunStatusCompletedString      = "completed"
	taskRunStatusCanceledString       = "canceled"
	taskRunStatusNeedsAttentionString = "needs_attention"
)

const (
	// TaskRunStatusUnknown is the zero value used before normalization.
	TaskRunStatusUnknown RunStatus = iota
	// TaskRunStatusQueued reports a run that has been accepted but not yet claimed.
	TaskRunStatusQueued
	// TaskRunStatusClaimed reports a run that has been claimed for execution.
	TaskRunStatusClaimed
	// TaskRunStatusStarting reports a run that is starting its execution session.
	TaskRunStatusStarting
	// TaskRunStatusRunning reports a run that is actively executing.
	TaskRunStatusRunning
	// TaskRunStatusCompleted reports a run that finished successfully.
	TaskRunStatusCompleted
	// TaskRunStatusFailed reports a run that finished with an error.
	TaskRunStatusFailed
	// TaskRunStatusCanceled reports a run that was canceled.
	TaskRunStatusCanceled
	// TaskRunStatusNeedsAttention reports a nonterminal run that requires operator or agent recovery.
	TaskRunStatusNeedsAttention
)

// String returns the durable string representation of the task-run status.
func (s RunStatus) String() string {
	switch s {
	case TaskRunStatusQueued:
		return taskRunStatusQueuedString
	case TaskRunStatusClaimed:
		return taskRunStatusClaimedString
	case TaskRunStatusStarting:
		return taskRunStatusStartingString
	case TaskRunStatusRunning:
		return string(InspectNextActionRunning)
	case TaskRunStatusCompleted:
		return taskRunStatusCompletedString
	case TaskRunStatusFailed:
		return string(TaskStatusFailed)
	case TaskRunStatusCanceled:
		return taskRunStatusCanceledString
	case TaskRunStatusNeedsAttention:
		return taskRunStatusNeedsAttentionString
	default:
		return ""
	}
}

// MarshalJSON encodes run status as the public/durable string value.
func (s RunStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes the public/durable run status string.
func (s *RunStatus) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed := ParseRunStatus(value)
	if err := parsed.Validate("task_run.status"); err != nil {
		return err
	}
	*s = parsed
	return nil
}

// RunKind identifies which executor owns a task-run body.
type RunKind uint8

const (
	// RunKindUnknown is the zero value used before normalization.
	RunKindUnknown RunKind = iota
	// RunKindWorker identifies normal ACP-backed worker runs.
	RunKindWorker
	// RunKindCoordinator identifies in-daemon generation coordinator runs.
	RunKindCoordinator
	// RunKindNetworkWake identifies durable Network admission work without a task anchor.
	RunKindNetworkWake
)

// String returns the durable string representation of the task-run kind.
func (k RunKind) String() string {
	switch k {
	case RunKindWorker:
		return "worker"
	case RunKindCoordinator:
		return "coordinator"
	case RunKindNetworkWake:
		return "network_wake"
	default:
		return ""
	}
}

// MarshalJSON encodes run kind as the public/durable string value.
func (k RunKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// UnmarshalJSON decodes the public/durable run kind string.
func (k *RunKind) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed := ParseRunKind(value)
	if err := parsed.Validate("task_run.run_kind"); err != nil {
		return err
	}
	*k = parsed
	return nil
}

const (
	// FailureKindOperatorForced identifies an operator-authored forced terminal failure.
	FailureKindOperatorForced = "operator_forced"
	// MaxForceRunBulkIDs bounds per-request bulk recovery work.
	MaxForceRunBulkIDs = 50
	// MaxRetryRunChainDepth bounds linear retry lineage to prevent accidental retry loops.
	MaxRetryRunChainDepth = 10
	// DefaultForceRunRateLimitPerMinute bounds force operations by actor and task.
	DefaultForceRunRateLimitPerMinute = 10
)

// ActorKind identifies the authenticated principal class behind task writes.
type ActorKind string

const (
	// ActorKindHuman identifies a human principal writing through CLI, web, HTTP, or UDS surfaces.
	ActorKindHuman ActorKind = "human"
	// ActorKindAgentSession identifies an AGH agent session principal.
	ActorKindAgentSession ActorKind = "agent_session"
	// ActorKindAutomation identifies daemon-owned automation flows.
	ActorKindAutomation ActorKind = "automation"
	// ActorKindExtension identifies an authenticated extension runtime principal.
	ActorKindExtension ActorKind = "extension"
	// ActorKindNetworkPeer identifies an authenticated network peer principal.
	ActorKindNetworkPeer ActorKind = "network_peer"
	// ActorKindDaemon identifies daemon-owned system work.
	ActorKindDaemon ActorKind = "daemon"
)

// OwnerKind identifies who currently owns a task operationally.
type OwnerKind string

const (
	// OwnerKindHuman identifies a human owner.
	OwnerKindHuman OwnerKind = "human"
	// OwnerKindAgentSession identifies an agent-session owner.
	OwnerKindAgentSession OwnerKind = "agent_session"
	// OwnerKindAutomation identifies an automation owner.
	OwnerKindAutomation OwnerKind = "automation"
	// OwnerKindExtension identifies an extension owner.
	OwnerKindExtension OwnerKind = "extension"
	// OwnerKindNetworkPeer identifies a network-peer owner.
	OwnerKindNetworkPeer OwnerKind = "network_peer"
	// OwnerKindPool identifies pooled ownership without a dedicated assignee.
	OwnerKindPool OwnerKind = "pool"
)

// OriginKind identifies the technical ingress surface that produced a task-domain write.
type OriginKind string

const (
	// OriginKindCLI identifies CLI ingress.
	OriginKindCLI OriginKind = "cli"
	// OriginKindWeb identifies web UI ingress.
	OriginKindWeb OriginKind = "web"
	// OriginKindUDS identifies local UDS ingress.
	OriginKindUDS OriginKind = "uds"
	// OriginKindHTTP identifies HTTP ingress.
	OriginKindHTTP OriginKind = "http"
	// OriginKindAutomation identifies automation ingress.
	OriginKindAutomation OriginKind = "automation"
	// OriginKindExtension identifies extension ingress.
	OriginKindExtension OriginKind = "extension"
	// OriginKindNetwork identifies network ingress.
	OriginKindNetwork OriginKind = "network"
	// OriginKindAgentSession identifies session tool-call ingress.
	OriginKindAgentSession OriginKind = "agent_session"
	// OriginKindDaemon identifies daemon-owned internal ingress.
	OriginKindDaemon OriginKind = "daemon"
)

// DependencyKind identifies the semantic meaning of one dependency edge.
type DependencyKind string

const (
	// DependencyKindBlocks identifies a dependency that must resolve before the task may proceed.
	DependencyKindBlocks DependencyKind = "blocks"
)

// BlockKind identifies a runtime-declared reason a task cannot proceed.
type BlockKind string

const (
	// BlockKindNeedsInput reports that the task needs human or creator input.
	BlockKindNeedsInput BlockKind = "needs_input"
	// BlockKindCapability reports that the task needs a capability that is not currently available.
	BlockKindCapability BlockKind = "capability"
	// BlockKindTransient reports a temporary external or environmental blocker.
	BlockKindTransient BlockKind = "transient"
)

// BlockedSource identifies one source in the derived blocked-reasons projection.
type BlockedSource string

const (
	// BlockedSourceDependency reports unresolved task dependency edges.
	BlockedSourceDependency BlockedSource = "dependency"
	// BlockedSourceApproval reports a pending approval gate.
	BlockedSourceApproval BlockedSource = "approval"
	// BlockedSourcePaused reports an operator pause gate.
	BlockedSourcePaused BlockedSource = "paused"
	// BlockedSourceBlock reports an open runtime-declared task block.
	BlockedSourceBlock BlockedSource = "block"
)

// StopReason identifies why the task domain asked the session bridge to stop a session.
type StopReason string

const (
	// StopReasonCompleted identifies successful task-run completion.
	StopReasonCompleted StopReason = "completed"
	// StopReasonFailed identifies failed task-run termination.
	StopReasonFailed StopReason = "failed"
	// StopReasonCancellation identifies explicit task or run cancellation.
	StopReasonCancellation StopReason = "cancellation"
	// StopReasonShutdown identifies daemon shutdown or boot recovery stop requests.
	StopReasonShutdown StopReason = "shutdown"
	// StopReasonOrphanedRun identifies orphaned-run recovery handling.
	StopReasonOrphanedRun StopReason = "orphaned_run"
)

// RunBootRecoveryAction identifies the manager-owned recovery action applied to
// a non-terminal run during daemon startup reconciliation.
type RunBootRecoveryAction string

const (
	// RunBootRecoveryRequeue resets one claimed run back to the durable queue.
	RunBootRecoveryRequeue RunBootRecoveryAction = "requeue"
	// RunBootRecoveryMarkRunning promotes one live attached run into running.
	RunBootRecoveryMarkRunning RunBootRecoveryAction = "mark_running"
	// RunBootRecoveryFail marks one orphaned attached run as failed.
	RunBootRecoveryFail RunBootRecoveryAction = "fail"
)
