package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskReferencePayload is the human-meaningful task identity shared across task read models.
type TaskReferencePayload struct {
	ID              string             `json:"id"`
	Identifier      string             `json:"identifier,omitempty"`
	Title           string             `json:"title"`
	Status          taskpkg.Status     `json:"status"`
	Priority        taskpkg.Priority   `json:"priority,omitempty"`
	Owner           *taskpkg.Ownership `json:"owner,omitempty"`
	Scope           taskpkg.Scope      `json:"scope"`
	WorkspaceID     string             `json:"workspace_id,omitempty"`
	LatestEventSeq  int64              `json:"latest_event_seq"`
	Paused          bool               `json:"paused,omitempty"`
	EffectivePaused bool               `json:"effective_paused,omitempty"`
	PausedByTaskID  string             `json:"paused_by_task_id,omitempty"`
}

// TaskSummaryPayload is the shared list-oriented task response payload.
type TaskSummaryPayload struct {
	ID                           string                           `json:"id"`
	Identifier                   string                           `json:"identifier,omitempty"`
	Scope                        taskpkg.Scope                    `json:"scope"`
	WorkspaceID                  string                           `json:"workspace_id,omitempty"`
	ParentTaskID                 string                           `json:"parent_task_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec              `json:"resolved_network_participation,omitempty"`
	Title                        string                           `json:"title"`
	Priority                     taskpkg.Priority                 `json:"priority,omitempty"`
	MaxAttempts                  int                              `json:"max_attempts,omitempty"`
	AutoEnqueueOnReady           bool                             `json:"auto_enqueue_on_ready,omitempty"`
	Status                       taskpkg.Status                   `json:"status"`
	ApprovalPolicy               taskpkg.ApprovalPolicy           `json:"approval_policy,omitempty"`
	ApprovalState                taskpkg.ApprovalState            `json:"approval_state,omitempty"`
	Draft                        bool                             `json:"draft,omitempty"`
	Owner                        *taskpkg.Ownership               `json:"owner,omitempty"`
	CurrentRunID                 string                           `json:"current_run_id,omitempty"`
	LatestEventSeq               int64                            `json:"latest_event_seq"`
	Paused                       bool                             `json:"paused,omitempty"`
	PausedBy                     string                           `json:"paused_by,omitempty"`
	PausedAt                     *time.Time                       `json:"paused_at,omitempty"`
	PausedReason                 string                           `json:"paused_reason,omitempty"`
	EffectivePaused              bool                             `json:"effective_paused,omitempty"`
	PausedByTaskID               string                           `json:"paused_by_task_id,omitempty"`
	BlockedReasons               []taskpkg.BlockedReason          `json:"blocked_reasons,omitempty"`
	NeedsAttention               bool                             `json:"needs_attention,omitempty"`
	NeedsAttentionReason         string                           `json:"needs_attention_reason,omitempty"`
	NeedsAttentionAt             *time.Time                       `json:"needs_attention_at,omitempty"`
	NeedsAttentionBy             *taskpkg.ActorIdentity           `json:"needs_attention_by,omitempty"`
	WakeCreator                  bool                             `json:"wake_creator"`
	CreatedBy                    taskpkg.ActorIdentity            `json:"created_by"`
	Origin                       taskpkg.Origin                   `json:"origin"`
	CreatedAt                    time.Time                        `json:"created_at"`
	UpdatedAt                    time.Time                        `json:"updated_at"`
	ClosedAt                     *time.Time                       `json:"closed_at,omitempty"`
	ChildCount                   int                              `json:"child_count,omitempty"`
	DependencyCount              int                              `json:"dependency_count,omitempty"`
	Dependencies                 []TaskDependencyReferencePayload `json:"dependencies,omitempty"`
	ActiveRun                    *TaskRunSummaryPayload           `json:"active_run,omitempty"`
	LastActivityAt               *time.Time                       `json:"last_activity_at,omitempty"`
}

// TaskPayload is the shared full task response payload.
type TaskPayload struct {
	ID                           string                 `json:"id"`
	Identifier                   string                 `json:"identifier,omitempty"`
	Scope                        taskpkg.Scope          `json:"scope"`
	WorkspaceID                  string                 `json:"workspace_id,omitempty"`
	ParentTaskID                 string                 `json:"parent_task_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec    `json:"resolved_network_participation,omitempty"`
	Title                        string                 `json:"title"`
	Description                  string                 `json:"description,omitempty"`
	Priority                     taskpkg.Priority       `json:"priority,omitempty"`
	MaxAttempts                  int                    `json:"max_attempts,omitempty"`
	AutoEnqueueOnReady           bool                   `json:"auto_enqueue_on_ready,omitempty"`
	Status                       taskpkg.Status         `json:"status"`
	ApprovalPolicy               taskpkg.ApprovalPolicy `json:"approval_policy,omitempty"`
	ApprovalState                taskpkg.ApprovalState  `json:"approval_state,omitempty"`
	Draft                        bool                   `json:"draft,omitempty"`
	Owner                        *taskpkg.Ownership     `json:"owner,omitempty"`
	CurrentRunID                 string                 `json:"current_run_id,omitempty"`
	LatestEventSeq               int64                  `json:"latest_event_seq"`
	Paused                       bool                   `json:"paused,omitempty"`
	PausedBy                     string                 `json:"paused_by,omitempty"`
	PausedAt                     *time.Time             `json:"paused_at,omitempty"`
	PausedReason                 string                 `json:"paused_reason,omitempty"`
	EffectivePaused              bool                   `json:"effective_paused,omitempty"`
	PausedByTaskID               string                 `json:"paused_by_task_id,omitempty"`
	// BlockedReasons is populated on read/detail projections and may be omitted by mutation responses.
	BlockedReasons       []taskpkg.BlockedReason `json:"blocked_reasons,omitempty"`
	NeedsAttention       bool                    `json:"needs_attention,omitempty"`
	NeedsAttentionReason string                  `json:"needs_attention_reason,omitempty"`
	NeedsAttentionAt     *time.Time              `json:"needs_attention_at,omitempty"`
	NeedsAttentionBy     *taskpkg.ActorIdentity  `json:"needs_attention_by,omitempty"`
	WakeCreator          bool                    `json:"wake_creator"`
	CreatedBy            taskpkg.ActorIdentity   `json:"created_by"`
	Origin               taskpkg.Origin          `json:"origin"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
	ClosedAt             *time.Time              `json:"closed_at,omitempty"`
	Metadata             json.RawMessage         `json:"metadata,omitempty"`
}

// TaskExecutionProfilePayload is the task-owned orchestration profile read model.
type TaskExecutionProfilePayload = taskpkg.ExecutionProfile

// SetTaskExecutionProfileRequest captures one profile replacement request.
type SetTaskExecutionProfileRequest = taskpkg.ExecutionProfile

// TaskExecutionProfileResponse wraps one execution profile response.
type TaskExecutionProfileResponse struct {
	Profile TaskExecutionProfilePayload `json:"profile"`
}

// TaskRunReviewPayload is the task-run review gate read model.
type TaskRunReviewPayload = taskpkg.RunReview

// CreateTaskRunReviewRequest captures one request to review a terminal task run.
type CreateTaskRunReviewRequest = taskpkg.RunReviewRequest

// SubmitTaskRunReviewVerdictRequest captures one persisted reviewer verdict write.
type SubmitTaskRunReviewVerdictRequest struct {
	RunID   string                   `json:"run_id"`
	Verdict taskpkg.RunReviewVerdict `json:"verdict"`
}

// TaskRunReviewListQuery captures shared review read filters.
type TaskRunReviewListQuery = taskpkg.RunReviewQuery

// TaskDependencyPayload is the shared dependency-edge response payload.
type TaskDependencyPayload struct {
	TaskID          string                 `json:"task_id"`
	DependsOnTaskID string                 `json:"depends_on_task_id"`
	Kind            taskpkg.DependencyKind `json:"kind"`
	CreatedAt       time.Time              `json:"created_at"`
}

// TaskBlockPayload is the public task-block read model.
type TaskBlockPayload struct {
	ID          string                 `json:"id"`
	TaskID      string                 `json:"task_id"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Kind        taskpkg.BlockKind      `json:"kind"`
	Reason      string                 `json:"reason"`
	Details     json.RawMessage        `json:"details,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	CreatedBy   taskpkg.ActorIdentity  `json:"created_by"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	ClearedAt   *time.Time             `json:"cleared_at,omitempty"`
	ClearedBy   *taskpkg.ActorIdentity `json:"cleared_by,omitempty"`
	ClearNote   string                 `json:"clear_note,omitempty"`
}

// TaskBlockResponse wraps one task-block payload.
type TaskBlockResponse struct {
	Block TaskBlockPayload `json:"block"`
}

// TaskBlocksResponse wraps a task-block list.
type TaskBlocksResponse struct {
	Blocks []TaskBlockPayload `json:"blocks"`
}

// TaskDependencyReferencePayload enriches one dependency edge with the referenced blocker identity.
type TaskDependencyReferencePayload struct {
	TaskID          string                 `json:"task_id"`
	DependsOnTaskID string                 `json:"depends_on_task_id"`
	Kind            taskpkg.DependencyKind `json:"kind"`
	CreatedAt       time.Time              `json:"created_at"`
	DependsOn       TaskReferencePayload   `json:"depends_on"`
}

// TaskEventPayload is the shared task audit-event response payload.
type TaskEventPayload struct {
	ID        string                `json:"id"`
	TaskID    string                `json:"task_id"`
	RunID     string                `json:"run_id,omitempty"`
	EventType string                `json:"event_type"`
	Actor     taskpkg.ActorIdentity `json:"actor"`
	Origin    taskpkg.Origin        `json:"origin"`
	Payload   json.RawMessage       `json:"payload,omitempty"`
	Timestamp time.Time             `json:"timestamp"`
}

// TaskDesignationRollupPayload exposes the persisted summary for one designated run group.
type TaskDesignationRollupPayload struct {
	DesignationGroupID string          `json:"designation_group_id"`
	TaskID             string          `json:"task_id"`
	Summary            json.RawMessage `json:"summary"`
	CreatedAt          time.Time       `json:"created_at"`
}

// TaskDetailPayload is the shared expanded task response payload.
type TaskDetailPayload struct {
	Summary              TaskSummaryPayload               `json:"summary"`
	Task                 TaskPayload                      `json:"task"`
	Children             []TaskSummaryPayload             `json:"children,omitempty"`
	Dependencies         []TaskDependencyPayload          `json:"dependencies,omitempty"`
	DependencyReferences []TaskDependencyReferencePayload `json:"dependency_references,omitempty"`
	Runs                 []TaskRunPayload                 `json:"runs,omitempty"`
	DesignationRollups   []TaskDesignationRollupPayload   `json:"designation_rollups,omitempty"`
	Events               []TaskEventPayload               `json:"events,omitempty"`
}

// TaskTimelineItemPayload is the shared task-timeline response payload.
type TaskTimelineItemPayload struct {
	Sequence  int64                  `json:"sequence"`
	EventID   string                 `json:"event_id"`
	Task      TaskReferencePayload   `json:"task"`
	Run       *TaskRunSummaryPayload `json:"run,omitempty"`
	EventType string                 `json:"event_type"`
	Actor     taskpkg.ActorIdentity  `json:"actor"`
	Origin    taskpkg.Origin         `json:"origin"`
	Payload   json.RawMessage        `json:"payload,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// TaskStreamEventPayload is one task-scoped replayable stream event.
type TaskStreamEventPayload struct {
	Sequence int64                   `json:"sequence"`
	Type     string                  `json:"type"`
	Timeline TaskTimelineItemPayload `json:"timeline"`
}

// TaskTreeNodePayload is one node inside a task-tree live snapshot.
type TaskTreeNodePayload struct {
	Task           TaskReferencePayload   `json:"task"`
	ParentTaskID   string                 `json:"parent_task_id,omitempty"`
	Depth          int                    `json:"depth"`
	ChildCount     int                    `json:"child_count,omitempty"`
	ActiveRun      *TaskRunSummaryPayload `json:"active_run,omitempty"`
	LastActivityAt time.Time              `json:"last_activity_at"`
}

// TaskTreePayload is the shared task-tree live snapshot.
type TaskTreePayload struct {
	Root        TaskTreeNodePayload   `json:"root"`
	Descendants []TaskTreeNodePayload `json:"descendants,omitempty"`
}

// TaskRunSessionPayload links one task run to its backing session when available.
type TaskRunSessionPayload struct {
	SessionID   string    `json:"session_id"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	AgentName   string    `json:"agent_name,omitempty"`
	Name        string    `json:"name,omitempty"`
	State       string    `json:"state,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskRunDetailPayload is the shared run-detail response payload.
type TaskRunDetailPayload struct {
	Run     TaskRunPayload                   `json:"run"`
	Task    *TaskReferencePayload            `json:"task,omitempty"`
	Session *TaskRunSessionPayload           `json:"session,omitempty"`
	Summary TaskRunOperationalSummaryPayload `json:"summary"`
	Network *TaskRunNetworkPayload           `json:"network,omitempty"`
}

// TaskInspectRunPayload is the redacted run projection returned by task inspect.
type TaskInspectRunPayload struct {
	RunID                   string            `json:"run_id"`
	TaskID                  string            `json:"task_id"`
	Status                  taskpkg.RunStatus `json:"status"`
	ClaimTokenHashTruncated string            `json:"claim_token_hash_truncated,omitempty"`
	LeaseUntil              *time.Time        `json:"lease_until,omitempty"`
	HeartbeatAt             *time.Time        `json:"heartbeat_at,omitempty"`
	HeartbeatAgeSeconds     *int64            `json:"heartbeat_age_seconds,omitempty"`
	Retries                 int               `json:"retries,omitempty"`
	LastErrorSummary        string            `json:"last_error_summary,omitempty"`
	FailureKind             string            `json:"failure_kind,omitempty"`
	BoundSessionID          string            `json:"bound_session_id,omitempty"`
	StartedAt               *time.Time        `json:"started_at,omitempty"`
	EndedAt                 *time.Time        `json:"ended_at,omitempty"`
	PreviousRunID           string            `json:"previous_run_id,omitempty"`
	QueuedAt                time.Time         `json:"queued_at"`
	Attempt                 int               `json:"attempt"`
}

// TaskInspectSessionPayload is the bound-session projection returned by task inspect.
type TaskInspectSessionPayload struct {
	SessionID      string     `json:"session_id"`
	State          string     `json:"state,omitempty"`
	AgentName      string     `json:"agent_name,omitempty"`
	ProviderName   string     `json:"provider_name,omitempty"`
	WorkspaceID    string     `json:"workspace_id,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
	StopReason     string     `json:"stop_reason,omitempty"`
	FailureKind    string     `json:"failure_kind,omitempty"`
}

// TaskInspectEventPayload is one recent event summary returned by task inspect.
type TaskInspectEventPayload struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	RunID     string    `json:"run_id,omitempty"`
	Outcome   string    `json:"outcome,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// TaskInspectSchedulerPayload is the scheduler state used for inspect diagnostics.
type TaskInspectSchedulerPayload struct {
	Paused    bool       `json:"paused"`
	PausedBy  string     `json:"paused_by,omitempty"`
	PausedAt  *time.Time `json:"paused_at,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// TaskInspectPayload is the shared task/run inspection response payload.
type TaskInspectPayload struct {
	Target       string                      `json:"target"`
	Task         TaskSummaryPayload          `json:"task"`
	CurrentRun   *TaskInspectRunPayload      `json:"current_run,omitempty"`
	BoundSession *TaskInspectSessionPayload  `json:"bound_session,omitempty"`
	RecentRuns   []TaskInspectRunPayload     `json:"recent_runs,omitempty"`
	RecentEvents []TaskInspectEventPayload   `json:"recent_events,omitempty"`
	Scheduler    TaskInspectSchedulerPayload `json:"scheduler"`
	Diagnostics  []DiagnosticItem            `json:"diagnostics,omitempty"`
	NextAction   string                      `json:"next_action"`
	AsOf         time.Time                   `json:"as_of"`
}

// TaskInspectResponse wraps one task/run inspect snapshot.
type TaskInspectResponse struct {
	Inspect TaskInspectPayload `json:"inspect"`
}
