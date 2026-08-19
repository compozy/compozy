package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRunRepo) heartbeatRunLeaseWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	heartbeat taskpkg.LeaseHeartbeat,
) (taskpkg.Run, error) {
	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, heartbeat.RunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireCurrentRunLease(current, heartbeat.ClaimToken, heartbeat.Now); err != nil {
		return taskpkg.Run{}, err
	}
	leaseUntil := heartbeat.Now.Add(heartbeat.LeaseDuration).UTC()
	affected, err := sqlcgen.New(exec).HeartbeatTaskRunLease(ctx, sqlcgen.HeartbeatTaskRunLeaseParams{
		LeaseUntil:     nullableTaskTime(leaseUntil),
		HeartbeatAt:    nullableTaskTime(heartbeat.Now),
		ClaimToken:     nullableTaskString(heartbeat.ClaimToken),
		TokensUsed:     heartbeat.TokensUsed,
		ID:             heartbeat.RunID,
		ClaimTokenHash: nullableTaskString(current.ClaimTokenHash),
		ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
		StartingStatus: taskpkg.TaskRunStatusStarting.String(),
		RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
	})
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: heartbeat task run lease %q: %w", heartbeat.RunID, err)
	}
	if affected == 0 {
		return taskpkg.Run{}, fmt.Errorf(
			"store: task run lease %q: %w",
			heartbeat.RunID,
			taskpkg.ErrTaskRunNotFound,
		)
	}
	updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, heartbeat.RunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if updated.IsNetworkWake() {
		wakeID, targetSessionID, ownerKey := updated.NetworkWakeCorrelation()
		if err := appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
			workspaceID: updated.WorkspaceID, wakeID: wakeID, taskRunID: updated.ID,
			ownerKey: ownerKey, targetSessionID: targetSessionID,
			eventType: networkWakeEventHeartbeat, state: updated.Status.String(),
			claimTokenHash: updated.ClaimTokenHash, actor: heartbeat.Actor.Actor, at: heartbeat.Now,
		}); err != nil {
			return taskpkg.Run{}, err
		}
		return updated, nil
	}
	if err := g.appendLoopTokenTickForHeartbeat(ctx, exec, updated, heartbeat.TokensUsed); err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}

func (g *TaskRunRepo) releaseRunLeaseWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	release taskpkg.LeaseRelease,
) (taskpkg.Run, error) {
	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, release.RunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireCurrentRunLease(current, release.ClaimToken, release.Now); err != nil {
		return taskpkg.Run{}, err
	}
	if err := requeueLeasedRun(ctx, exec, current.ID); err != nil {
		return taskpkg.Run{}, err
	}
	if current.IsTaskAnchored() {
		if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
			return taskpkg.Run{}, err
		}
	}
	updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, current.ID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if updated.IsNetworkWake() {
		wakeID, targetSessionID, ownerKey := updated.NetworkWakeCorrelation()
		if err := appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
			workspaceID: updated.WorkspaceID, wakeID: wakeID, taskRunID: updated.ID,
			ownerKey: ownerKey, targetSessionID: targetSessionID,
			eventType: networkWakeEventReleased, state: updated.Status.String(),
			reason: release.Reason, actor: release.Actor.Actor, at: release.Now,
		}); err != nil {
			return taskpkg.Run{}, err
		}
	}
	return updated, nil
}

func (g *TaskRunRepo) bindLeasedRunSessionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	binding taskpkg.LeaseSessionBinding,
) (taskpkg.Run, error) {
	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, binding.RunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireCurrentRunLease(current, binding.ClaimToken, binding.Now); err != nil {
		return taskpkg.Run{}, err
	}
	affected, err := sqlcgen.New(exec).BindLeasedTaskRunSession(ctx, sqlcgen.BindLeasedTaskRunSessionParams{
		SessionID:      nullableTaskString(binding.SessionID),
		ID:             binding.RunID,
		ClaimTokenHash: nullableTaskString(current.ClaimTokenHash),
		ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
		StartingStatus: taskpkg.TaskRunStatusStarting.String(),
		RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
	})
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: bind leased task run session %q: %w", binding.RunID, err)
	}
	if affected == 0 {
		return taskpkg.Run{}, fmt.Errorf(
			"store: task run lease %q: %w",
			binding.RunID,
			taskpkg.ErrTaskRunNotFound,
		)
	}
	return g.tasks.getTaskRunWithExecutor(ctx, exec, binding.RunID)
}

func (g *TaskRunRepo) failRunLeaseMutationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	failure taskpkg.LeaseFailure,
) (taskpkg.FailedRunLeaseMutation, error) {
	normalized, err := failure.Normalize(g.now())
	if err != nil {
		return taskpkg.FailedRunLeaseMutation{}, err
	}
	if err := normalized.Actor.Validate(); err != nil {
		return taskpkg.FailedRunLeaseMutation{}, err
	}

	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
	if err != nil {
		return taskpkg.FailedRunLeaseMutation{}, err
	}
	if current.IsNetworkWake() {
		return taskpkg.FailedRunLeaseMutation{}, fmt.Errorf(
			"%w: network_wake runs must be failed through network settlement",
			taskpkg.ErrValidation,
		)
	}
	return g.failCurrentRunLeaseWithExecutor(ctx, exec, current, normalized)
}

func (g *TaskRunRepo) failRunLeaseWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	failure taskpkg.LeaseFailure,
) (taskpkg.Run, error) {
	normalized, err := failure.Normalize(g.now())
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := normalized.Actor.Validate(); err != nil {
		return taskpkg.Run{}, err
	}
	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	mutation, err := g.failCurrentRunLeaseWithExecutor(ctx, exec, current, normalized)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return mutation.Run, nil
}

func (g *TaskRunRepo) failCurrentRunLeaseWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Run,
	normalized taskpkg.LeaseFailure,
) (taskpkg.FailedRunLeaseMutation, error) {
	if err := requireCurrentRunLease(current, normalized.ClaimToken, normalized.Now); err != nil {
		return taskpkg.FailedRunLeaseMutation{}, err
	}
	if err := taskpkg.RequireLeaseSettlementActor(current, normalized.Actor); err != nil {
		return taskpkg.FailedRunLeaseMutation{}, err
	}
	if err := requireLeaseTerminalTransition(current, taskpkg.TaskRunStatusFailed); err != nil {
		return taskpkg.FailedRunLeaseMutation{}, err
	}
	if current.RunKind.Normalize() == taskpkg.RunKindCoordinator {
		normalized.Failure = taskpkg.CanonicalCoordinatorRunFailure(normalized.Failure)
	}

	affected, err := sqlcgen.New(exec).FailTaskRunLease(ctx, sqlcgen.FailTaskRunLeaseParams{
		Status:         taskpkg.TaskRunStatusFailed.String(),
		EndedAt:        nullableTaskTime(normalized.Now),
		TokensUsed:     normalized.TokensUsed,
		Error:          nullableTaskString(normalized.Failure.Error),
		ID:             current.ID,
		ClaimTokenHash: nullableTaskString(current.ClaimTokenHash),
	})
	if err != nil {
		return taskpkg.FailedRunLeaseMutation{}, fmt.Errorf(
			"store: fail task run lease %q: %w",
			current.ID,
			err,
		)
	}
	if affected == 0 {
		return taskpkg.FailedRunLeaseMutation{}, fmt.Errorf(
			"store: task run lease %q: %w",
			current.ID,
			taskpkg.ErrTaskRunNotFound,
		)
	}
	if current.IsTaskAnchored() {
		if err := settleCoordinatorFailureLoopWithExecutor(ctx, exec, current, normalized); err != nil {
			return taskpkg.FailedRunLeaseMutation{}, err
		}
		if err := recordLoopNodeTerminalWithExecutor(
			ctx,
			exec,
			current,
			"failure",
			loopFailureOutputRef(normalized.Failure),
			nil,
			normalized.Now,
		); err != nil {
			return taskpkg.FailedRunLeaseMutation{}, err
		}
		if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
			return taskpkg.FailedRunLeaseMutation{}, err
		}
	}

	updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, current.ID)
	if err != nil {
		return taskpkg.FailedRunLeaseMutation{}, err
	}
	return taskpkg.FailedRunLeaseMutation{
		Run:     updated,
		Failure: normalized.Failure,
	}, nil
}
