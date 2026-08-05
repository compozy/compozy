//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type fakeStore struct{}

func (store fakeStore) WithTaskExecutionTransaction(
	_ context.Context,
	fn func(taskpkg.ExecutionMutationStore) error,
) error {
	return fn(store)
}

func (store fakeStore) WithLeaseSettlementTransaction(
	_ context.Context,
	_ string,
	fn func(taskpkg.LeaseSettlementMutationStore) error,
) error {
	return fn(store)
}

func (store fakeStore) WithTaskMutationTransaction(
	_ context.Context,
	_ string,
	mutation taskpkg.RunMutation,
) error {
	return mutation(store)
}

func (fakeStore) CreateTask(context.Context, taskpkg.Task) error { return nil }

func (fakeStore) DeleteTask(context.Context, string) error { return nil }

func (fakeStore) UpdateTask(context.Context, taskpkg.Task, taskpkg.ActorContext) error { return nil }
func (fakeStore) CreateTaskDefinition(
	context.Context,
	taskpkg.CreateTaskDefinitionMutation,
) error {
	return nil
}
func (fakeStore) UpdateTaskDefinition(
	context.Context,
	*taskpkg.UpdateTaskDefinitionMutation,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, nil
}
func (fakeStore) SetTaskExecutionProfile(
	context.Context,
	*taskpkg.ExecutionProfileMutation,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, nil
}
func (fakeStore) DeleteTaskExecutionProfile(
	context.Context,
	taskpkg.ExecutionProfileDeleteMutation,
) error {
	return nil
}

func (fakeStore) GetTask(context.Context, string) (taskpkg.Task, error) { return taskpkg.Task{}, nil }

func (fakeStore) ListTasks(context.Context, taskpkg.Query) ([]taskpkg.Summary, error) {
	return []taskpkg.Summary{{
		ID:             "task-1",
		Title:          "bootstrap",
		Scope:          taskpkg.ScopeGlobal,
		Priority:       taskpkg.PriorityMedium,
		MaxAttempts:    taskpkg.DefaultTaskMaxAttempts,
		ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		ApprovalState:  taskpkg.ApprovalStatePending,
	}}, nil
}

func (fakeStore) CountDirectChildren(context.Context, string) (int, error) { return 0, nil }

func (fakeStore) CreateDependency(context.Context, taskpkg.Dependency) error { return nil }

func (fakeStore) DeleteDependency(context.Context, string, string) error { return nil }

func (fakeStore) ListDependencies(context.Context, string) ([]taskpkg.Dependency, error) {
	return []taskpkg.Dependency{{TaskID: "task-1", DependsOnTaskID: "task-0", Kind: taskpkg.DependencyKindBlocks}}, nil
}

func (fakeStore) ListDependents(context.Context, string) ([]taskpkg.Dependency, error) {
	return nil, nil
}

func (fakeStore) CountDependencies(context.Context, string) (int, error) { return 1, nil }

func (fakeStore) HasDependencyPath(context.Context, string, string) (bool, error) { return false, nil }

func (fakeStore) ListTaskBlocks(context.Context, string, bool) ([]taskpkg.TaskBlock, error) {
	return nil, nil
}

func (fakeStore) HasOpenTaskBlocks(context.Context, string) (bool, error) {
	return false, nil
}

func (fakeStore) CreateTaskBlock(
	context.Context,
	taskpkg.CreateTaskBlockMutation,
) (taskpkg.BlockMutationResult, error) {
	return taskpkg.BlockMutationResult{}, nil
}

func (fakeStore) ClearTaskBlock(context.Context, taskpkg.ClearTaskBlockMutation) (taskpkg.TaskBlock, error) {
	return taskpkg.TaskBlock{}, nil
}

func (fakeStore) ClearTaskNeedsAttention(
	context.Context,
	taskpkg.NeedsAttentionClearMutation,
) (taskpkg.NeedsAttentionClearResult, error) {
	return taskpkg.NeedsAttentionClearResult{}, nil
}

func (fakeStore) ListExpiredTaskBlockTargets(
	context.Context,
	time.Time,
) ([]taskpkg.BlockExpiryTarget, error) {
	return nil, nil
}

func (fakeStore) ExpireTaskBlocks(
	context.Context,
	taskpkg.ExpireTaskBlocksMutation,
) (taskpkg.ExpireTaskBlocksResult, error) {
	return taskpkg.ExpireTaskBlocksResult{}, nil
}

func (fakeStore) BlockTaskAndReleaseRun(
	context.Context,
	taskpkg.BlockTaskAndReleaseRunMutation,
) (taskpkg.BlockTaskAndReleaseRunResult, error) {
	return taskpkg.BlockTaskAndReleaseRunResult{}, nil
}

func (fakeStore) UpdateTaskRunMetadata(
	context.Context,
	taskpkg.RunMetadataMutation,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) FailTasklessRunOnBoot(
	context.Context,
	taskpkg.TerminalRunMutation,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) TransitionTerminalRun(
	_ context.Context,
	mutation taskpkg.TerminalRunMutation,
) (taskpkg.Run, error) {
	return mutation.NextRun(), nil
}

func (fakeStore) SettleCompletedTaskHierarchy(
	context.Context,
	string,
	taskpkg.ActorContext,
	time.Time,
) (taskpkg.CompletedRunSettlement, error) {
	return taskpkg.CompletedRunSettlement{}, nil
}

func (fakeStore) ReserveTerminalRunCommand(
	_ context.Context,
	command taskpkg.TerminalRunCommand,
) (taskpkg.TerminalRunCommand, bool, error) {
	return command, true, nil
}

func (fakeStore) AdvanceTerminalRunCommandPhase(
	_ context.Context,
	command taskpkg.TerminalRunCommand,
	_ taskpkg.TerminalRunCommandPhase,
	_ time.Time,
) (taskpkg.TerminalRunCommand, error) {
	return command, nil
}

func (fakeStore) ReleaseTerminalRunCommand(
	context.Context,
	taskpkg.TerminalRunCommand,
) error {
	return nil
}

func (fakeStore) ListTerminalRunCommands(context.Context) ([]taskpkg.TerminalRunCommand, error) {
	return nil, nil
}

func (fakeStore) ConsumeTerminalRunCommand(
	_ context.Context,
	command taskpkg.TerminalRunCommand,
	_ taskpkg.TerminalRunCommandPhase,
) (taskpkg.TerminalRunCommand, error) {
	return command, nil
}

func (fakeStore) AdmitRunDirectExecution(
	context.Context,
	taskpkg.RunDirectExecutionAdmissionMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return taskpkg.NominalRunMutationResult{}, nil
}

func (fakeStore) TransitionRunStarting(
	context.Context,
	taskpkg.RunStartingMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return taskpkg.NominalRunMutationResult{}, nil
}

func (fakeStore) BindRunSession(
	context.Context,
	taskpkg.RunSessionBindingMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return taskpkg.NominalRunMutationResult{}, nil
}

func (fakeStore) TransitionRunRunning(
	context.Context,
	taskpkg.RunRunningMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return taskpkg.NominalRunMutationResult{}, nil
}

func (fakeStore) RecoverTaskRunOnBoot(
	context.Context,
	taskpkg.RunBootRecoveryMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return taskpkg.NominalRunMutationResult{}, nil
}

func (fakeStore) RecoverNetworkWakeOnBoot(
	context.Context,
	taskpkg.NetworkWakeBootRecoveryMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return taskpkg.NominalRunMutationResult{}, nil
}

func (fakeStore) GetTaskRun(context.Context, string) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) ListTaskRuns(context.Context, taskpkg.RunQuery) ([]taskpkg.Run, error) {
	return []taskpkg.Run{{ID: "run-1", TaskID: "task-1", Status: taskpkg.TaskRunStatusQueued, Attempt: 1}}, nil
}

func (fakeStore) ListTaskRunsByStatus(context.Context, []taskpkg.RunStatus) ([]taskpkg.Run, error) {
	return []taskpkg.Run{{ID: "run-1", TaskID: "task-1", Status: taskpkg.TaskRunStatusQueued, Attempt: 1}}, nil
}

func (fakeStore) ClaimNextRun(context.Context, taskpkg.ClaimCriteria) (taskpkg.ClaimResult, error) {
	return taskpkg.ClaimResult{}, taskpkg.ErrNoClaimableRun
}

func (fakeStore) HeartbeatRunLease(context.Context, taskpkg.LeaseHeartbeat) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) ReleaseRunLease(context.Context, taskpkg.LeaseRelease) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) ListActiveSessionRunLeases(context.Context, string) ([]taskpkg.Run, error) {
	return nil, nil
}

func (fakeStore) ReleaseSessionRunLease(
	context.Context,
	taskpkg.Run,
	taskpkg.SessionLeaseRelease,
	taskpkg.ActorContext,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) CompleteRunLease(context.Context, taskpkg.LeaseCompletion) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) CompleteRunLeaseSettlement(
	context.Context,
	taskpkg.LeaseCompletion,
) (taskpkg.CompletedRunSettlement, error) {
	return taskpkg.CompletedRunSettlement{}, nil
}

func (fakeStore) FailRunLease(context.Context, taskpkg.LeaseFailure) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) FailRunLeaseMutation(
	context.Context,
	taskpkg.LeaseFailure,
) (taskpkg.FailedRunLeaseMutation, error) {
	return taskpkg.FailedRunLeaseMutation{}, nil
}

func (fakeStore) SettleNetworkWake(
	context.Context,
	taskpkg.NetworkWakeSettlement,
) (taskpkg.NetworkWakeSettlementResult, error) {
	return taskpkg.NetworkWakeSettlementResult{}, nil
}

func (fakeStore) MarkRunNeedsAttentionMutation(
	context.Context,
	taskpkg.RunNeedsAttentionCommand,
) (taskpkg.RunNeedsAttentionMutation, error) {
	return taskpkg.RunNeedsAttentionMutation{}, nil
}

func (fakeStore) ForceReleaseTaskRun(
	context.Context,
	taskpkg.ForceReleaseRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	return taskpkg.ForceRunMutationResult{}, nil
}

func (fakeStore) ForceFailTaskRun(
	context.Context,
	taskpkg.ForceFailRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	return taskpkg.ForceRunMutationResult{}, nil
}

func (fakeStore) RetryTaskRun(context.Context, taskpkg.RetryRunMutation) (taskpkg.RetryRunResult, error) {
	return taskpkg.RetryRunResult{}, nil
}

func (fakeStore) RecoverTaskRun(context.Context, taskpkg.RecoverRunMutation) (taskpkg.RetryRunResult, error) {
	return taskpkg.RetryRunResult{}, nil
}

func (fakeStore) InvalidateForceRunInputs(
	context.Context,
	string,
	time.Time,
) (taskpkg.ForceRunInputInvalidation, error) {
	return taskpkg.ForceRunInputInvalidation{}, nil
}

func (fakeStore) RecoverExpiredRunLeases(
	context.Context,
	taskpkg.ExpiredLeaseRecovery,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	return nil, nil
}

func (fakeStore) ReserveQueuedRun(
	context.Context,
	taskpkg.QueueRunReservation,
) (taskpkg.Task, taskpkg.Run, bool, error) {
	return taskpkg.Task{
			ID:             "task-1",
			Scope:          taskpkg.ScopeGlobal,
			Title:          "bootstrap",
			Priority:       taskpkg.PriorityMedium,
			MaxAttempts:    taskpkg.DefaultTaskMaxAttempts,
			Status:         taskpkg.TaskStatusReady,
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
			ApprovalState:  taskpkg.ApprovalStatePending,
		}, taskpkg.Run{
			ID:      "run-1",
			TaskID:  "task-1",
			Status:  taskpkg.TaskRunStatusQueued,
			Attempt: 1,
		}, false, nil
}

func (fakeStore) CompleteCoordinatorAndEnqueueNext(
	context.Context,
	taskpkg.CoordinatorCompletion,
	taskpkg.GenerationStateFinalizer,
) (taskpkg.CoordinatorCompletionResult, error) {
	return taskpkg.CoordinatorCompletionResult{}, nil
}

func (fakeStore) GetTaskTriageState(context.Context, string, taskpkg.ActorIdentity) (taskpkg.TriageState, error) {
	return taskpkg.TriageState{}, taskpkg.ErrTaskTriageStateNotFound
}

func (fakeStore) UpsertTaskTriageState(context.Context, taskpkg.TriageState) error { return nil }

func (fakeStore) GetExecutionProfile(context.Context, string) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
}

func (fakeStore) UpsertExecutionProfile(
	context.Context,
	*taskpkg.ExecutionProfile,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, nil
}

func (fakeStore) DeleteExecutionProfile(context.Context, string) error {
	return taskpkg.ErrExecutionProfileNotFound
}

func (fakeStore) RequestRunReview(
	context.Context,
	*taskpkg.RunReview,
) (taskpkg.RunReview, bool, error) {
	return taskpkg.RunReview{}, false, taskpkg.ErrRunReviewNotFound
}

func (fakeStore) GetRunReview(context.Context, string) (taskpkg.RunReview, error) {
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (fakeStore) RecordRunReview(
	context.Context,
	taskpkg.RecordRunReviewRequest,
	taskpkg.ActorContext,
	time.Time,
	string,
) (taskpkg.RunReviewResult, error) {
	return taskpkg.RunReviewResult{}, taskpkg.ErrRunReviewNotFound
}

func (fakeStore) BindRunReviewSession(
	context.Context,
	taskpkg.BindRunReviewSessionRequest,
	time.Time,
) (taskpkg.RunReview, error) {
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (fakeStore) LookupRunReviewBySession(context.Context, string) (taskpkg.RunReview, error) {
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (fakeStore) ListRunReviews(context.Context, taskpkg.RunReviewQuery) ([]taskpkg.RunReview, error) {
	return nil, nil
}

func (fakeStore) CountActiveSessionBindings(context.Context, string) (int, error) { return 0, nil }

func (fakeStore) CreateTaskEvent(context.Context, taskpkg.Event) error { return nil }

func (fakeStore) ListTaskEvents(context.Context, taskpkg.EventQuery) ([]taskpkg.Event, error) {
	return []taskpkg.Event{{ID: "evt-1", TaskID: "task-1", EventType: "task.created"}}, nil
}

func (fakeStore) TaskWakeEventExists(context.Context, string, string) (bool, error) {
	return false, nil
}

func (fakeStore) GetTaskEventRecord(context.Context, string) (taskpkg.EventRecord, error) {
	return taskpkg.EventRecord{
		Sequence: 1,
		Event:    taskpkg.Event{ID: "evt-1", TaskID: "task-1", EventType: "task.created"},
	}, nil
}

func (fakeStore) ListTaskEventRecords(
	context.Context,
	taskpkg.EventRecordQuery,
) ([]taskpkg.EventRecord, error) {
	return []taskpkg.EventRecord{{
		Sequence: 1,
		Event:    taskpkg.Event{ID: "evt-1", TaskID: "task-1", EventType: "task.created"},
	}}, nil
}

func (fakeStore) GetTaskRunByIdempotencyKey(context.Context, string, taskpkg.Origin) (taskpkg.Run, error) {
	return taskpkg.Run{}, nil
}

func (fakeStore) SaveTaskRunIdempotency(context.Context, taskpkg.RunIdempotency) error {
	return nil
}

type fakeSessionExecutor struct{}

func (fakeSessionExecutor) StartTaskSession(context.Context, *taskpkg.StartTaskSession) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: "sess-1"}, nil
}

func (fakeSessionExecutor) AttachTaskSession(context.Context, string, string) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: "sess-1"}, nil
}

func (fakeSessionExecutor) RequestTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (fakeSessionExecutor) ForceTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

type fakeCoordinator struct {
	store    taskpkg.Store
	sessions taskpkg.SessionExecutor
}

func (c fakeCoordinator) compose(ctx context.Context) error {
	if _, err := c.store.ListTasks(ctx, taskpkg.Query{
		Limit:         1,
		Priority:      taskpkg.PriorityMedium,
		ApprovalState: taskpkg.ApprovalStatePending,
	}); err != nil {
		return err
	}
	if err := c.sessions.RequestTaskStop(ctx, "sess-1", taskpkg.StopReasonCancellation); err != nil {
		return err
	}
	return nil
}

var _ taskpkg.Store = (*fakeStore)(nil)
var _ taskpkg.SessionExecutor = (*fakeSessionExecutor)(nil)

func TestTaskDomainInterfacesComposeWithoutSessionImport(t *testing.T) {
	t.Parallel()

	coordinator := fakeCoordinator{
		store:    fakeStore{},
		sessions: fakeSessionExecutor{},
	}
	if err := coordinator.compose(context.Background()); err != nil {
		t.Fatalf("compose() error = %v", err)
	}
}
