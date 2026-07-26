package testutil

import (
	"context"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

type forceReleaseRunFunc func(
	context.Context,
	string,
	taskpkg.ForceReleaseRun,
	taskpkg.ActorContext,
) (*taskpkg.Run, error)

type forceFailRunFunc func(
	context.Context,
	string,
	taskpkg.ForceFailRun,
	taskpkg.ActorContext,
) (*taskpkg.Run, error)

type retryRunFunc func(
	context.Context,
	string,
	taskpkg.RetryRunRequest,
	taskpkg.ActorContext,
) (*taskpkg.RetryRunResult, error)

type recoverRunFunc func(
	context.Context,
	string,
	taskpkg.RecoverRunRequest,
	taskpkg.ActorContext,
) (*taskpkg.RetryRunResult, error)

type bulkForceRunFunc func(
	context.Context,
	taskpkg.BulkForceRunRequest,
	taskpkg.ActorContext,
) (taskpkg.BulkForceRunResult, error)

type pauseTaskFunc func(
	context.Context,
	string,
	taskpkg.PauseTaskRequest,
	taskpkg.ActorContext,
) (*taskpkg.Task, error)

type resumeTaskFunc func(
	context.Context,
	string,
	taskpkg.ResumeTaskRequest,
	taskpkg.ActorContext,
) (*taskpkg.Task, error)

type StubTaskManager struct {
	CreateTaskFn      func(context.Context, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error)
	CreateChildTaskFn func(
		context.Context,
		string,
		taskpkg.CreateTask,
		taskpkg.ActorContext,
	) (*taskpkg.Task, error)
	DeleteTaskFn  func(context.Context, string, taskpkg.ActorContext) error
	UpdateTaskFn  func(context.Context, string, taskpkg.Patch, taskpkg.ActorContext) (*taskpkg.Task, error)
	PublishTaskFn func(
		context.Context,
		string,
		taskpkg.ExecutionRequest,
		taskpkg.ActorContext,
	) (*taskpkg.Execution, error)
	StartTaskFn func(
		context.Context,
		string,
		taskpkg.ExecutionRequest,
		taskpkg.ActorContext,
	) (*taskpkg.Execution, error)
	ApproveTaskFn func(
		context.Context,
		string,
		taskpkg.ExecutionRequest,
		taskpkg.ActorContext,
	) (*taskpkg.Execution, error)
	RejectTaskFn func(context.Context, string, taskpkg.ActorContext) (*taskpkg.Task, error)
	CancelTaskFn func(
		context.Context,
		string,
		taskpkg.CancelTask,
		taskpkg.ActorContext,
	) (*taskpkg.Task, error)
	BlockTaskFn      func(context.Context, taskpkg.BlockRequest, taskpkg.ActorContext) (taskpkg.TaskBlock, error)
	ClearTaskBlockFn func(
		context.Context,
		string,
		string,
		string,
		taskpkg.ActorContext,
	) (taskpkg.TaskBlock, error)
	RecoverTaskFn         func(context.Context, string, string, taskpkg.ActorContext) (*taskpkg.Task, error)
	ExpireTaskBlocksFn    func(context.Context, time.Time, taskpkg.ActorContext) (taskpkg.ExpireTaskBlocksResult, error)
	ListTaskBlocksFn      func(context.Context, string, bool, taskpkg.ActorContext) ([]taskpkg.TaskBlock, error)
	MarkTaskReadFn        func(context.Context, string, taskpkg.ActorContext) (taskpkg.TriageState, error)
	ArchiveTaskFn         func(context.Context, string, taskpkg.ActorContext) (taskpkg.TriageState, error)
	DismissTaskFn         func(context.Context, string, taskpkg.ActorContext) (taskpkg.TriageState, error)
	GetExecutionProfileFn func(
		context.Context,
		string,
		taskpkg.ActorContext,
	) (taskpkg.ExecutionProfile, error)
	SetExecutionProfileFn func(
		context.Context,
		string,
		*taskpkg.ExecutionProfile,
		taskpkg.ActorContext,
	) (taskpkg.ExecutionProfile, error)
	DeleteExecutionProfileFn func(context.Context, string, taskpkg.ActorContext) error
	RequestRunReviewFn       func(
		context.Context,
		taskpkg.RunReviewRequest,
		taskpkg.ActorContext,
	) (taskpkg.RunReview, bool, error)
	GetRunReviewFn    func(context.Context, string, taskpkg.ActorContext) (taskpkg.RunReview, error)
	RecordRunReviewFn func(
		context.Context,
		taskpkg.RecordRunReviewRequest,
		taskpkg.ActorContext,
	) (taskpkg.RunReviewResult, error)
	BindRunReviewSessionFn func(
		context.Context,
		taskpkg.BindRunReviewSessionRequest,
		taskpkg.ActorContext,
	) (taskpkg.RunReviewBinding, error)
	ListRunReviewsFn   func(context.Context, taskpkg.RunReviewQuery, taskpkg.ActorContext) ([]taskpkg.RunReview, error)
	AddDependencyFn    func(context.Context, taskpkg.AddDependency, taskpkg.ActorContext) error
	RemoveDependencyFn func(context.Context, string, string, taskpkg.ActorContext) error
	EnqueueRunFn       func(context.Context, taskpkg.EnqueueRun, taskpkg.ActorContext) (*taskpkg.Run, error)
	ClaimNextRunFn     func(
		context.Context,
		taskpkg.ClaimCriteria,
		taskpkg.ActorContext,
	) (*taskpkg.ClaimResult, error)
	StartRunFn             func(context.Context, string, taskpkg.StartRun, taskpkg.ActorContext) (*taskpkg.Run, error)
	AttachRunSessionFn     func(context.Context, string, string, taskpkg.ActorContext) (*taskpkg.Run, error)
	HeartbeatRunLeaseFn    func(context.Context, taskpkg.LeaseHeartbeat, taskpkg.ActorContext) (*taskpkg.Run, error)
	ReleaseRunLeaseFn      func(context.Context, taskpkg.LeaseRelease, taskpkg.ActorContext) (*taskpkg.Run, error)
	ForceReleaseRunFn      forceReleaseRunFunc
	ForceFailRunFn         forceFailRunFunc
	RetryRunFn             retryRunFunc
	RecoverRunFn           recoverRunFunc
	BulkForceReleaseRunsFn bulkForceRunFunc
	BulkForceFailRunsFn    bulkForceRunFunc
	PauseTaskFn            pauseTaskFunc
	ResumeTaskFn           resumeTaskFunc
	SchedulerStatusFn      func(context.Context, taskpkg.ActorContext) (taskpkg.SchedulerStatus, error)
	PauseSchedulerFn       func(
		context.Context,
		taskpkg.SchedulerPauseRequest,
		taskpkg.ActorContext,
	) (taskpkg.SchedulerStatus, error)
	ResumeSchedulerFn func(
		context.Context,
		taskpkg.SchedulerResumeRequest,
		taskpkg.ActorContext,
	) (taskpkg.SchedulerStatus, error)
	DrainSchedulerFn func(
		context.Context,
		taskpkg.SchedulerDrainRequest,
		taskpkg.ActorContext,
	) (taskpkg.SchedulerDrainResult, error)
	SchedulerBacklogFn func(
		context.Context,
		taskpkg.SchedulerBacklogQuery,
		taskpkg.ActorContext,
	) (taskpkg.SchedulerBacklog, error)
	CompleteRunLeaseFn  func(context.Context, taskpkg.LeaseCompletion, taskpkg.ActorContext) (*taskpkg.Run, error)
	FailRunLeaseFn      func(context.Context, taskpkg.LeaseFailure, taskpkg.ActorContext) (*taskpkg.Run, error)
	SettleNetworkWakeFn func(
		context.Context,
		taskpkg.NetworkWakeSettlement,
		taskpkg.ActorContext,
	) (*taskpkg.NetworkWakeSettlementResult, error)
	LookupActiveRunForSessionFn func(
		context.Context,
		string,
		string,
	) (taskpkg.AutonomyLeaseHandle, error)
	CompleteRunFn             func(context.Context, string, taskpkg.RunResult, taskpkg.ActorContext) (*taskpkg.Run, error)
	FailRunFn                 func(context.Context, string, taskpkg.RunFailure, taskpkg.ActorContext) (*taskpkg.Run, error)
	CancelRunFn               func(context.Context, string, taskpkg.CancelRun, taskpkg.ActorContext) (*taskpkg.Run, error)
	RecoverExpiredRunLeasesFn func(
		context.Context,
		taskpkg.ExpiredLeaseRecovery,
		taskpkg.ActorContext,
	) ([]taskpkg.ExpiredLeaseRecoveryResult, error)
	GetTaskFn         func(context.Context, string, taskpkg.ActorContext) (*taskpkg.View, error)
	InspectTaskFn     func(context.Context, string, taskpkg.ActorContext) (*taskpkg.InspectView, error)
	InspectRunFn      func(context.Context, string, taskpkg.ActorContext) (*taskpkg.InspectView, error)
	ListTaskRunsFn    func(context.Context, string, taskpkg.RunQuery, taskpkg.ActorContext) ([]taskpkg.Run, error)
	ListTasksFn       func(context.Context, taskpkg.Query, taskpkg.ActorContext) ([]taskpkg.Summary, error)
	ListTaskCatalogFn func(
		context.Context,
		taskpkg.CatalogQuery,
		taskpkg.ActorContext,
	) (taskpkg.CatalogPage, error)
	TimelineFn func(
		context.Context,
		string,
		taskpkg.TimelineQuery,
		taskpkg.ActorContext,
	) ([]taskpkg.TimelineItem, error)
	StreamFn func(
		context.Context,
		string,
		taskpkg.StreamQuery,
		taskpkg.ActorContext,
	) (<-chan taskpkg.StreamEvent, error)
	TreeFn      func(context.Context, string, taskpkg.ActorContext) (*taskpkg.TreeView, error)
	RunDetailFn func(context.Context, string, taskpkg.ActorContext) (*taskpkg.RunDetailView, error)
}

func (s *StubTaskManager) CreateTask(
	ctx context.Context,
	spec taskpkg.CreateTask,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.CreateTaskFn != nil {
		return s.CreateTaskFn(ctx, spec, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) CreateChildTask(
	ctx context.Context,
	parentTaskID string,
	spec taskpkg.CreateTask,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.CreateChildTaskFn != nil {
		return s.CreateChildTaskFn(ctx, parentTaskID, spec, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) DeleteTask(
	ctx context.Context,
	id string,
	actor taskpkg.ActorContext,
) error {
	if s.DeleteTaskFn != nil {
		return s.DeleteTaskFn(ctx, id, actor)
	}
	return taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) UpdateTask(
	ctx context.Context,
	id string,
	patch taskpkg.Patch,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.UpdateTaskFn != nil {
		return s.UpdateTaskFn(ctx, id, patch, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) PublishTask(
	ctx context.Context,
	id string,
	req taskpkg.ExecutionRequest,
	actor taskpkg.ActorContext,
) (*taskpkg.Execution, error) {
	if s.PublishTaskFn != nil {
		return s.PublishTaskFn(ctx, id, req, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) StartTask(
	ctx context.Context,
	id string,
	req taskpkg.ExecutionRequest,
	actor taskpkg.ActorContext,
) (*taskpkg.Execution, error) {
	if s.StartTaskFn != nil {
		return s.StartTaskFn(ctx, id, req, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) ApproveTask(
	ctx context.Context,
	id string,
	req taskpkg.ExecutionRequest,
	actor taskpkg.ActorContext,
) (*taskpkg.Execution, error) {
	if s.ApproveTaskFn != nil {
		return s.ApproveTaskFn(ctx, id, req, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) RejectTask(
	ctx context.Context,
	id string,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.RejectTaskFn != nil {
		return s.RejectTaskFn(ctx, id, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) CancelTask(
	ctx context.Context,
	id string,
	req taskpkg.CancelTask,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.CancelTaskFn != nil {
		return s.CancelTaskFn(ctx, id, req, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) BlockTask(
	ctx context.Context,
	req taskpkg.BlockRequest,
	actor taskpkg.ActorContext,
) (taskpkg.TaskBlock, error) {
	if s.BlockTaskFn != nil {
		return s.BlockTaskFn(ctx, req, actor)
	}
	return taskpkg.TaskBlock{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) ClearTaskBlock(
	ctx context.Context,
	taskID string,
	blockID string,
	note string,
	actor taskpkg.ActorContext,
) (taskpkg.TaskBlock, error) {
	if s.ClearTaskBlockFn != nil {
		return s.ClearTaskBlockFn(ctx, taskID, blockID, note, actor)
	}
	return taskpkg.TaskBlock{}, taskpkg.ErrTaskBlockNotFound
}

func (s *StubTaskManager) RecoverTask(
	ctx context.Context,
	id string,
	note string,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.RecoverTaskFn != nil {
		return s.RecoverTaskFn(ctx, id, note, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) ExpireTaskBlocks(
	ctx context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (taskpkg.ExpireTaskBlocksResult, error) {
	if s.ExpireTaskBlocksFn != nil {
		return s.ExpireTaskBlocksFn(ctx, now, actor)
	}
	return taskpkg.ExpireTaskBlocksResult{}, nil
}

func (s *StubTaskManager) ListTaskBlocks(
	ctx context.Context,
	taskID string,
	includeCleared bool,
	actor taskpkg.ActorContext,
) ([]taskpkg.TaskBlock, error) {
	if s.ListTaskBlocksFn != nil {
		return s.ListTaskBlocksFn(ctx, taskID, includeCleared, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) MarkTaskRead(
	ctx context.Context,
	id string,
	actor taskpkg.ActorContext,
) (taskpkg.TriageState, error) {
	if s.MarkTaskReadFn != nil {
		return s.MarkTaskReadFn(ctx, id, actor)
	}
	return taskpkg.TriageState{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) ArchiveTask(
	ctx context.Context,
	id string,
	actor taskpkg.ActorContext,
) (taskpkg.TriageState, error) {
	if s.ArchiveTaskFn != nil {
		return s.ArchiveTaskFn(ctx, id, actor)
	}
	return taskpkg.TriageState{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) DismissTask(
	ctx context.Context,
	id string,
	actor taskpkg.ActorContext,
) (taskpkg.TriageState, error) {
	if s.DismissTaskFn != nil {
		return s.DismissTaskFn(ctx, id, actor)
	}
	return taskpkg.TriageState{}, taskpkg.ErrTaskNotFound
}
