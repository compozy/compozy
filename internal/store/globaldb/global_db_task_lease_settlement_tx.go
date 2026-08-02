package globaldb

import (
	"context"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type taskLeaseSettlementTxStore struct {
	*taskExecutionTxStore
}

var _ taskpkg.LeaseSettlementMutationStore = (*taskLeaseSettlementTxStore)(nil)
var _ taskpkg.LeaseSettlementTransactionStore = (*TaskRepo)(nil)
var _ taskpkg.LeaseSettlementTransactionStore = (*GlobalDB)(nil)

// WithLeaseSettlementTransaction runs one lease settlement under the repository's writer transaction.
func (g *TaskRepo) WithLeaseSettlementTransaction(
	ctx context.Context,
	action string,
	fn func(taskpkg.LeaseSettlementMutationStore) error,
) error {
	if err := g.checkReady(ctx, action); err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, action, func(exec taskSQLExecutor) error {
		store := &taskLeaseSettlementTxStore{
			taskExecutionTxStore: &taskExecutionTxStore{
				deleteTaskTxStore: &deleteTaskTxStore{tasks: g, exec: exec},
			},
		}
		return fn(store)
	})
}

func (s *taskLeaseSettlementTxStore) ClaimNextRun(
	ctx context.Context,
	criteria taskpkg.ClaimCriteria,
) (taskpkg.ClaimResult, error) {
	normalized, err := criteria.Normalize(s.tasks.now())
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	return s.tasks.runs.claimNextRunWithExecutor(ctx, s.exec, normalized)
}

func (s *taskLeaseSettlementTxStore) HeartbeatRunLease(
	ctx context.Context,
	heartbeat taskpkg.LeaseHeartbeat,
) (taskpkg.Run, error) {
	normalized, err := heartbeat.Normalize(s.tasks.now())
	if err != nil {
		return taskpkg.Run{}, err
	}
	return s.tasks.runs.heartbeatRunLeaseWithExecutor(ctx, s.exec, normalized)
}

func (s *taskLeaseSettlementTxStore) ReleaseRunLease(
	ctx context.Context,
	release taskpkg.LeaseRelease,
) (taskpkg.Run, error) {
	normalized, err := release.Normalize(s.tasks.now())
	if err != nil {
		return taskpkg.Run{}, err
	}
	return s.tasks.runs.releaseRunLeaseWithExecutor(ctx, s.exec, normalized)
}

func (s *taskLeaseSettlementTxStore) FailRunLeaseMutation(
	ctx context.Context,
	failure taskpkg.LeaseFailure,
) (taskpkg.FailedRunLeaseMutation, error) {
	return s.tasks.runs.failRunLeaseMutationWithExecutor(ctx, s.exec, failure)
}

func (s *taskLeaseSettlementTxStore) GetTaskRun(
	ctx context.Context,
	runID string,
) (taskpkg.Run, error) {
	return s.tasks.getTaskRunWithExecutor(ctx, s.exec, runID)
}
