package testutil

import (
	"context"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *StubTaskManager) CompleteRunLease(
	ctx context.Context,
	completion taskpkg.LeaseCompletion,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.CompleteRunLeaseFn != nil {
		return s.CompleteRunLeaseFn(ctx, completion, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) FailRunLease(
	ctx context.Context,
	failure taskpkg.LeaseFailure,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.FailRunLeaseFn != nil {
		return s.FailRunLeaseFn(ctx, failure, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) SettleNetworkWake(
	ctx context.Context,
	settlement taskpkg.NetworkWakeSettlement,
	actor taskpkg.ActorContext,
) (*taskpkg.NetworkWakeSettlementResult, error) {
	if s.SettleNetworkWakeFn != nil {
		return s.SettleNetworkWakeFn(ctx, settlement, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) LookupActiveRunForSession(
	ctx context.Context,
	sessionID string,
	runID string,
) (taskpkg.AutonomyLeaseHandle, error) {
	if s.LookupActiveRunForSessionFn != nil {
		return s.LookupActiveRunForSessionFn(ctx, sessionID, runID)
	}
	return taskpkg.AutonomyLeaseHandle{}, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) CompleteRun(
	ctx context.Context,
	runID string,
	result taskpkg.RunResult,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.CompleteRunFn != nil {
		return s.CompleteRunFn(ctx, runID, result, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) FailRun(
	ctx context.Context,
	runID string,
	failure taskpkg.RunFailure,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.FailRunFn != nil {
		return s.FailRunFn(ctx, runID, failure, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) CancelRun(
	ctx context.Context,
	runID string,
	req taskpkg.CancelRun,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.CancelRunFn != nil {
		return s.CancelRunFn(ctx, runID, req, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) RecoverExpiredRunLeases(
	ctx context.Context,
	recovery taskpkg.ExpiredLeaseRecovery,
	actor taskpkg.ActorContext,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	if s.RecoverExpiredRunLeasesFn != nil {
		return s.RecoverExpiredRunLeasesFn(ctx, recovery, actor)
	}
	return nil, nil
}
