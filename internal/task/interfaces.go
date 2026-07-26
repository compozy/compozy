package task

import (
	"context"
	"time"
)

// Manager is the task-domain authority for task and run lifecycle operations.
type Manager interface {
	CreateTask(ctx context.Context, spec CreateTask, actor ActorContext) (*Task, error)
	CreateChildTask(ctx context.Context, parentTaskID string, spec CreateTask, actor ActorContext) (*Task, error)
	DeleteTask(ctx context.Context, id string, actor ActorContext) error
	UpdateTask(ctx context.Context, id string, patch Patch, actor ActorContext) (*Task, error)
	PublishTask(ctx context.Context, id string, req ExecutionRequest, actor ActorContext) (*Execution, error)
	StartTask(ctx context.Context, id string, req ExecutionRequest, actor ActorContext) (*Execution, error)
	ApproveTask(ctx context.Context, id string, req ExecutionRequest, actor ActorContext) (*Execution, error)
	RejectTask(ctx context.Context, id string, actor ActorContext) (*Task, error)
	CancelTask(ctx context.Context, id string, req CancelTask, actor ActorContext) (*Task, error)
	PauseTask(ctx context.Context, id string, req PauseTaskRequest, actor ActorContext) (*Task, error)
	ResumeTask(ctx context.Context, id string, req ResumeTaskRequest, actor ActorContext) (*Task, error)
	BlockTask(ctx context.Context, req BlockRequest, actor ActorContext) (TaskBlock, error)
	RecoverTask(ctx context.Context, id string, note string, actor ActorContext) (*Task, error)
	ClearTaskBlock(
		ctx context.Context,
		taskID string,
		blockID string,
		note string,
		actor ActorContext,
	) (TaskBlock, error)
	ExpireTaskBlocks(ctx context.Context, now time.Time, actor ActorContext) (ExpireTaskBlocksResult, error)
	ListTaskBlocks(ctx context.Context, taskID string, includeCleared bool, actor ActorContext) ([]TaskBlock, error)
	MarkTaskRead(ctx context.Context, id string, actor ActorContext) (TriageState, error)
	ArchiveTask(ctx context.Context, id string, actor ActorContext) (TriageState, error)
	DismissTask(ctx context.Context, id string, actor ActorContext) (TriageState, error)

	GetExecutionProfile(ctx context.Context, taskID string, actor ActorContext) (ExecutionProfile, error)
	SetExecutionProfile(
		ctx context.Context,
		taskID string,
		profile *ExecutionProfile,
		actor ActorContext,
	) (ExecutionProfile, error)
	DeleteExecutionProfile(ctx context.Context, taskID string, actor ActorContext) error

	RequestRunReview(ctx context.Context, req RunReviewRequest, actor ActorContext) (RunReview, bool, error)
	GetRunReview(ctx context.Context, reviewID string, actor ActorContext) (RunReview, error)
	RecordRunReview(ctx context.Context, req RecordRunReviewRequest, actor ActorContext) (RunReviewResult, error)
	BindRunReviewSession(
		ctx context.Context,
		req BindRunReviewSessionRequest,
		actor ActorContext,
	) (RunReviewBinding, error)
	LookupRunReviewForSession(ctx context.Context, sessionID string, actor ActorContext) (RunReviewBinding, error)
	ListRunReviews(ctx context.Context, query RunReviewQuery, actor ActorContext) ([]RunReview, error)

	AddDependency(ctx context.Context, spec AddDependency, actor ActorContext) error
	RemoveDependency(ctx context.Context, taskID string, dependsOnID string, actor ActorContext) error

	EnqueueRun(ctx context.Context, spec EnqueueRun, actor ActorContext) (*Run, error)
	ClaimNextRun(ctx context.Context, criteria ClaimCriteria, actor ActorContext) (*ClaimResult, error)
	StartRun(ctx context.Context, runID string, req StartRun, actor ActorContext) (*Run, error)
	AttachRunSession(ctx context.Context, runID string, sessionID string, actor ActorContext) (*Run, error)
	HeartbeatRunLease(ctx context.Context, heartbeat LeaseHeartbeat, actor ActorContext) (*Run, error)
	ReleaseRunLease(ctx context.Context, release LeaseRelease, actor ActorContext) (*Run, error)
	ForceReleaseRun(ctx context.Context, runID string, release ForceReleaseRun, actor ActorContext) (*Run, error)
	ForceFailRun(ctx context.Context, runID string, failure ForceFailRun, actor ActorContext) (*Run, error)
	RetryRun(ctx context.Context, runID string, retry RetryRunRequest, actor ActorContext) (*RetryRunResult, error)
	RecoverRun(
		ctx context.Context,
		runID string,
		req RecoverRunRequest,
		actor ActorContext,
	) (*RetryRunResult, error)
	BulkForceReleaseRuns(ctx context.Context, req BulkForceRunRequest, actor ActorContext) (BulkForceRunResult, error)
	BulkForceFailRuns(ctx context.Context, req BulkForceRunRequest, actor ActorContext) (BulkForceRunResult, error)
	CompleteRunLease(ctx context.Context, completion LeaseCompletion, actor ActorContext) (*Run, error)
	FailRunLease(ctx context.Context, failure LeaseFailure, actor ActorContext) (*Run, error)
	SettleNetworkWake(
		ctx context.Context,
		settlement NetworkWakeSettlement,
		actor ActorContext,
	) (*NetworkWakeSettlementResult, error)
	CompleteRun(ctx context.Context, runID string, result RunResult, actor ActorContext) (*Run, error)
	FailRun(ctx context.Context, runID string, failure RunFailure, actor ActorContext) (*Run, error)
	CancelRun(ctx context.Context, runID string, req CancelRun, actor ActorContext) (*Run, error)
	RecoverExpiredRunLeases(
		ctx context.Context,
		recovery ExpiredLeaseRecovery,
		actor ActorContext,
	) ([]ExpiredLeaseRecoveryResult, error)
	SchedulerStatus(ctx context.Context, actor ActorContext) (SchedulerStatus, error)
	PauseScheduler(ctx context.Context, req SchedulerPauseRequest, actor ActorContext) (SchedulerStatus, error)
	ResumeScheduler(ctx context.Context, req SchedulerResumeRequest, actor ActorContext) (SchedulerStatus, error)
	DrainScheduler(ctx context.Context, req SchedulerDrainRequest, actor ActorContext) (SchedulerDrainResult, error)
	SchedulerBacklog(ctx context.Context, query SchedulerBacklogQuery, actor ActorContext) (SchedulerBacklog, error)

	GetTask(ctx context.Context, id string, actor ActorContext) (*View, error)
	InspectTask(ctx context.Context, taskID string, actor ActorContext) (*InspectView, error)
	InspectRun(ctx context.Context, runID string, actor ActorContext) (*InspectView, error)
	ListTaskRuns(ctx context.Context, taskID string, query RunQuery, actor ActorContext) ([]Run, error)
	ListTasks(ctx context.Context, query Query, actor ActorContext) ([]Summary, error)
	ListTaskCatalog(ctx context.Context, query CatalogQuery, actor ActorContext) (CatalogPage, error)

	LiveService
}

// RecordStore is the persistence surface for durable task records.
type RecordStore interface {
	CreateTask(ctx context.Context, task Task) error
	DeleteTask(ctx context.Context, id string) error
	UpdateTask(ctx context.Context, task Task, actor ActorContext) error
	GetTask(ctx context.Context, id string) (Task, error)
	ListTasks(ctx context.Context, query Query) ([]Summary, error)
	CountDirectChildren(ctx context.Context, parentTaskID string) (int, error)
}

// DeleteTaskMutationStore is the narrowed persistence surface required to
// execute task deletion and dependent reconciliation as one unit.
type DeleteTaskMutationStore interface {
	BlockReader
	GetTask(ctx context.Context, id string) (Task, error)
	UpdateTask(ctx context.Context, task Task, actor ActorContext) error
	DeleteTask(ctx context.Context, id string) error
	CountDirectChildren(ctx context.Context, parentTaskID string) (int, error)
	ListDependencies(ctx context.Context, taskID string) ([]Dependency, error)
	ListDependents(ctx context.Context, dependsOnTaskID string) ([]Dependency, error)
	ListTaskRuns(ctx context.Context, query RunQuery) ([]Run, error)
}

// DeleteTaskTransactionStore optionally exposes transactional delete-task
// execution so the manager can roll back the primary delete when dependent
// reconciliation fails.
type DeleteTaskTransactionStore interface {
	WithDeleteTaskTransaction(ctx context.Context, fn func(DeleteTaskMutationStore) error) error
}

// DependencyStore is the persistence surface for durable dependency edges.
type DependencyStore interface {
	CreateDependency(ctx context.Context, dependency Dependency) error
	DeleteDependency(ctx context.Context, taskID string, dependsOnID string) error
	ListDependencies(ctx context.Context, taskID string) ([]Dependency, error)
	ListDependents(ctx context.Context, dependsOnTaskID string) ([]Dependency, error)
	CountDependencies(ctx context.Context, taskID string) (int, error)
	HasDependencyPath(ctx context.Context, fromTaskID string, toTaskID string) (bool, error)
}

// BlockReader is the persistence surface for read-only task-block projections.
type BlockReader interface {
	ListTaskBlocks(ctx context.Context, taskID string, includeCleared bool) ([]TaskBlock, error)
	HasOpenTaskBlocks(ctx context.Context, taskID string) (bool, error)
}

// BlockStore is the persistence surface for task-block lifecycle mutations.
type BlockStore interface {
	BlockReader
	CreateTaskBlock(ctx context.Context, mutation CreateTaskBlockMutation) (BlockMutationResult, error)
	ClearTaskBlock(ctx context.Context, mutation ClearTaskBlockMutation) (TaskBlock, error)
	ClearTaskNeedsAttention(
		ctx context.Context,
		mutation NeedsAttentionClearMutation,
	) (NeedsAttentionClearResult, error)
	ExpireTaskBlocks(ctx context.Context, mutation ExpireTaskBlocksMutation) (ExpireTaskBlocksResult, error)
	BlockTaskAndReleaseRun(
		ctx context.Context,
		mutation BlockTaskAndReleaseRunMutation,
	) (BlockTaskAndReleaseRunResult, error)
}

// RunStore is the persistence surface for durable task-run records.
type RunStore interface {
	CreateTaskRun(ctx context.Context, run Run) error
	UpdateTaskRun(ctx context.Context, run Run) error
	CompleteRunSettlement(ctx context.Context, run Run, actor ActorContext) (CompletedRunSettlement, error)
	GetTaskRun(ctx context.Context, id string) (Run, error)
	ListTaskRuns(ctx context.Context, query RunQuery) ([]Run, error)
	ListTaskRunsByStatus(ctx context.Context, statuses []RunStatus) ([]Run, error)
	CountActiveSessionBindings(ctx context.Context, sessionID string) (int, error)
	ClaimNextRun(ctx context.Context, criteria ClaimCriteria) (ClaimResult, error)
	HeartbeatRunLease(ctx context.Context, heartbeat LeaseHeartbeat) (Run, error)
	ReleaseRunLease(ctx context.Context, release LeaseRelease) (Run, error)
	CompleteRunLease(ctx context.Context, completion LeaseCompletion) (Run, error)
	CompleteRunLeaseSettlement(ctx context.Context, completion LeaseCompletion) (CompletedRunSettlement, error)
	FailRunLease(ctx context.Context, failure LeaseFailure) (Run, error)
	SettleNetworkWake(ctx context.Context, settlement NetworkWakeSettlement) (NetworkWakeSettlementResult, error)
	CompleteCoordinatorAndEnqueueNext(
		ctx context.Context,
		completion CoordinatorCompletion,
		finalizer GenerationStateFinalizer,
	) (CoordinatorCompletionResult, error)
	ForceReleaseTaskRun(ctx context.Context, release ForceReleaseRunMutation) (ForceRunMutationResult, error)
	ForceFailTaskRun(ctx context.Context, failure ForceFailRunMutation) (ForceRunMutationResult, error)
	RetryTaskRun(ctx context.Context, retry RetryRunMutation) (RetryRunResult, error)
	RecoverTaskRun(ctx context.Context, mutation RecoverRunMutation) (RetryRunResult, error)
	MarkTaskRunNeedsAttention(ctx context.Context, runID string, diagnostic string) (Run, error)
	// RecoverExpiredRunLeases commits each run mutation with its canonical recovery events.
	RecoverExpiredRunLeases(ctx context.Context, recovery ExpiredLeaseRecovery) ([]ExpiredLeaseRecoveryResult, error)
	ReserveQueuedRun(ctx context.Context, reservation QueueRunReservation) (Task, Run, bool, error)
}

// EventStore is the persistence surface for immutable task audit events.
type EventStore interface {
	CreateTaskEvent(ctx context.Context, event Event) error
	ListTaskEvents(ctx context.Context, query EventQuery) ([]Event, error)
	TaskWakeEventExists(ctx context.Context, taskID string, wakeEventID string) (bool, error)
}

// EventSequenceStore is the persistence surface for stable task event sequencing used by live reads.
type EventSequenceStore interface {
	GetTaskEventRecord(ctx context.Context, eventID string) (EventRecord, error)
	ListTaskEventRecords(ctx context.Context, query EventRecordQuery) ([]EventRecord, error)
}

// EventCommitObserverStore publishes immutable task events only after the
// transaction that owns them has committed.
type EventCommitObserverStore interface {
	SetTaskEventCommitObserver(observer EventObserver)
}

// IdempotencyStore is the persistence surface for non-human run idempotency tracking.
type IdempotencyStore interface {
	GetTaskRunByIdempotencyKey(ctx context.Context, key string, origin Origin) (Run, error)
	SaveTaskRunIdempotency(ctx context.Context, record RunIdempotency) error
}

// ExecutionMutationStore exposes the persistence operations that one
// publish/start/approve/enqueue command may use while holding a single writer
// transaction.
type ExecutionMutationStore interface {
	DeleteTaskMutationStore
	ExecutionProfileStore
	IdempotencyStore
	CreateTaskEvent(ctx context.Context, event Event) error
	ReserveQueuedRun(
		ctx context.Context,
		reservation QueueRunReservation,
	) (Task, Run, bool, error)
}

// ExecutionTransactionStore owns the atomic execution-command boundary.
type ExecutionTransactionStore interface {
	WithTaskExecutionTransaction(
		ctx context.Context,
		fn func(ExecutionMutationStore) error,
	) error
}

// TriageStore is the persistence surface for durable actor-scoped task triage state.
type TriageStore interface {
	GetTaskTriageState(ctx context.Context, taskID string, actor ActorIdentity) (TriageState, error)
	UpsertTaskTriageState(ctx context.Context, state TriageState) error
}

// ExecutionProfileStore is the persistence surface for task-owned execution profiles.
type ExecutionProfileStore interface {
	GetExecutionProfile(ctx context.Context, taskID string) (ExecutionProfile, error)
	UpsertExecutionProfile(ctx context.Context, profile *ExecutionProfile) (ExecutionProfile, error)
	DeleteExecutionProfile(ctx context.Context, taskID string) error
}

// RunReviewStore is the persistence surface for task-run review gate records.
type RunReviewStore interface {
	RequestRunReview(ctx context.Context, review *RunReview) (RunReview, bool, error)
	GetRunReview(ctx context.Context, reviewID string) (RunReview, error)
	RecordRunReview(
		ctx context.Context,
		req RecordRunReviewRequest,
		actor ActorContext,
		recordedAt time.Time,
		continuationRunID string,
	) (RunReviewResult, error)
	BindRunReviewSession(ctx context.Context, req BindRunReviewSessionRequest, boundAt time.Time) (RunReview, error)
	LookupRunReviewBySession(ctx context.Context, sessionID string) (RunReview, error)
	ListRunReviews(ctx context.Context, query RunReviewQuery) ([]RunReview, error)
}

// Store composes the task-domain persistence surfaces consumed by the manager.
type Store interface {
	RecordStore
	DefinitionStore
	DependencyStore
	BlockStore
	RunStore
	EventStore
	EventSequenceStore
	IdempotencyStore
	TriageStore
	ExecutionProfileStore
	RunReviewStore
}

// SessionExecutor is the injected runtime bridge used to start, attach, and stop task sessions.
type SessionExecutor interface {
	StartTaskSession(ctx context.Context, spec *StartTaskSession) (*SessionRef, error)
	AttachTaskSession(ctx context.Context, runID string, sessionID string) (*SessionRef, error)
	RequestTaskStop(ctx context.Context, sessionID string, reason StopReason) error
	ForceTaskStop(ctx context.Context, sessionID string, reason StopReason) error
}
