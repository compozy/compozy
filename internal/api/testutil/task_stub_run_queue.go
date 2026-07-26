package testutil

import (
	"context"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *StubTaskManager) EnqueueRun(
	ctx context.Context,
	spec taskpkg.EnqueueRun,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.EnqueueRunFn != nil {
		return s.EnqueueRunFn(ctx, spec, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) ClaimNextRun(
	ctx context.Context,
	criteria taskpkg.ClaimCriteria,
	actor taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	if s.ClaimNextRunFn != nil {
		return s.ClaimNextRunFn(ctx, criteria, actor)
	}
	return nil, taskpkg.ErrNoClaimableRun
}

func (s *StubTaskManager) StartRun(
	ctx context.Context,
	runID string,
	req taskpkg.StartRun,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.StartRunFn != nil {
		return s.StartRunFn(ctx, runID, req, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) AttachRunSession(
	ctx context.Context,
	runID string,
	sessionID string,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.AttachRunSessionFn != nil {
		return s.AttachRunSessionFn(ctx, runID, sessionID, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) HeartbeatRunLease(
	ctx context.Context,
	heartbeat taskpkg.LeaseHeartbeat,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.HeartbeatRunLeaseFn != nil {
		return s.HeartbeatRunLeaseFn(ctx, heartbeat, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) ReleaseRunLease(
	ctx context.Context,
	release taskpkg.LeaseRelease,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.ReleaseRunLeaseFn != nil {
		return s.ReleaseRunLeaseFn(ctx, release, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) ForceReleaseRun(
	ctx context.Context,
	runID string,
	release taskpkg.ForceReleaseRun,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.ForceReleaseRunFn != nil {
		return s.ForceReleaseRunFn(ctx, runID, release, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) ForceFailRun(
	ctx context.Context,
	runID string,
	failure taskpkg.ForceFailRun,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	if s.ForceFailRunFn != nil {
		return s.ForceFailRunFn(ctx, runID, failure, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) RetryRun(
	ctx context.Context,
	runID string,
	retry taskpkg.RetryRunRequest,
	actor taskpkg.ActorContext,
) (*taskpkg.RetryRunResult, error) {
	if s.RetryRunFn != nil {
		return s.RetryRunFn(ctx, runID, retry, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) RecoverRun(
	ctx context.Context,
	runID string,
	req taskpkg.RecoverRunRequest,
	actor taskpkg.ActorContext,
) (*taskpkg.RetryRunResult, error) {
	if s.RecoverRunFn != nil {
		return s.RecoverRunFn(ctx, runID, req, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) BulkForceReleaseRuns(
	ctx context.Context,
	req taskpkg.BulkForceRunRequest,
	actor taskpkg.ActorContext,
) (taskpkg.BulkForceRunResult, error) {
	if s.BulkForceReleaseRunsFn != nil {
		return s.BulkForceReleaseRunsFn(ctx, req, actor)
	}
	return taskpkg.BulkForceRunResult{}, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) BulkForceFailRuns(
	ctx context.Context,
	req taskpkg.BulkForceRunRequest,
	actor taskpkg.ActorContext,
) (taskpkg.BulkForceRunResult, error) {
	if s.BulkForceFailRunsFn != nil {
		return s.BulkForceFailRunsFn(ctx, req, actor)
	}
	return taskpkg.BulkForceRunResult{}, taskpkg.ErrTaskRunNotFound
}
