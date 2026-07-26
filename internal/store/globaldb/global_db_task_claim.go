package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	loop "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	globalDBTaskClaimStatusKey = "status"
)

const (
	globalDBTaskClaimHandoffKey = "handoff"
)

type taskRunLeaseSnapshot struct {
	status         taskpkg.RunStatus
	sessionID      string
	leaseUntil     time.Time
	claimTokenHash string
}

const taskPriorityValueSQL = `CASE t.priority
	WHEN 'urgent' THEN 40
	WHEN 'high' THEN 30
	WHEN 'low' THEN 10
	ELSE 20
END`

// ClaimNextRun atomically selects and claims the next eligible queued task run.
func (g *TaskRunRepo) ClaimNextRun(
	ctx context.Context,
	criteria taskpkg.ClaimCriteria,
) (taskpkg.ClaimResult, error) {
	if err := g.checkReady(ctx, "claim next task run"); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	normalized, err := criteria.Normalize(g.now())
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}

	var result taskpkg.ClaimResult
	if err := g.tasks.withTaskImmediateTransaction(ctx, "claim next task run", func(exec taskSQLExecutor) error {
		var claimErr error
		result, claimErr = g.claimNextRunWithExecutor(ctx, exec, normalized)
		return claimErr
	}); err != nil {
		return taskpkg.ClaimResult{}, err
	}

	return result, nil
}

func (g *TaskRunRepo) claimNextRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	criteria taskpkg.ClaimCriteria,
) (taskpkg.ClaimResult, error) {
	if err := g.ensureClaimerHasNoActiveLease(ctx, exec, criteria); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	runID, err := g.selectClaimableRunID(ctx, exec, criteria)
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	if runID == "" {
		return taskpkg.ClaimResult{}, taskpkg.ErrNoClaimableRun
	}
	if err := g.ensureWorkspaceActiveRunCapacity(ctx, exec, runID, criteria); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	claimToken, err := taskpkg.NewClaimToken()
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	claimHash, err := taskpkg.ClaimTokenHash(claimToken)
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	leaseUntil := criteria.Now.Add(criteria.LeaseDuration).UTC()
	if err := claimRunWithExecutor(
		ctx,
		exec,
		runID,
		criteria,
		claimToken,
		claimHash,
		leaseUntil,
	); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	run, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	if run.IsNetworkWake() {
		return claimNetworkWakeResult(ctx, exec, run, claimToken, leaseUntil, criteria.Now)
	}
	return g.claimStandardTaskRunResult(ctx, exec, run, claimToken, leaseUntil, criteria.Now)
}

func claimNetworkWakeResult(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	claimToken string,
	leaseUntil time.Time,
	claimedAt time.Time,
) (taskpkg.ClaimResult, error) {
	if run.ClaimedBy == nil {
		return taskpkg.ClaimResult{}, fmt.Errorf("store: network wake claim actor is required")
	}
	wakeID, targetSessionID, ownerKey := run.NetworkWakeCorrelation()
	if err := appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
		workspaceID: run.WorkspaceID, wakeID: wakeID, taskRunID: run.ID,
		ownerKey: ownerKey, targetSessionID: targetSessionID,
		eventType: networkWakeEventClaimed, state: run.Status.String(),
		claimTokenHash: run.ClaimTokenHash, actor: *run.ClaimedBy, at: claimedAt,
	}); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	return taskpkg.ClaimResult{Run: run, ClaimToken: claimToken, LeaseUntil: leaseUntil}, nil
}

func (g *TaskRunRepo) claimStandardTaskRunResult(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	claimToken string,
	leaseUntil time.Time,
	claimedAt time.Time,
) (taskpkg.ClaimResult, error) {
	if err := setTaskCurrentRunProjectionForRun(ctx, exec, run.ID); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	if err := appendLoopNodeRunningEventWithExecutor(ctx, exec, run, claimedAt); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	taskRecord, err := g.tasks.getTaskWithExecutor(ctx, exec, run.TaskID)
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	channel, err := g.coordinationChannelMetadata(ctx, exec, run)
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	return taskpkg.ClaimResult{
		Task:                &taskRecord,
		Run:                 run,
		ClaimToken:          claimToken,
		LeaseUntil:          leaseUntil,
		CoordinationChannel: channel,
	}, nil
}

// HeartbeatRunLease extends one active task-run lease after token verification.
func (g *TaskRunRepo) HeartbeatRunLease(
	ctx context.Context,
	heartbeat taskpkg.LeaseHeartbeat,
) (taskpkg.Run, error) {
	if err := g.checkReady(ctx, "heartbeat task run lease"); err != nil {
		return taskpkg.Run{}, err
	}
	normalized, err := heartbeat.Normalize(g.now())
	if err != nil {
		return taskpkg.Run{}, err
	}

	var updated taskpkg.Run
	if err := g.tasks.withTaskImmediateTransaction(ctx, "heartbeat task run lease", func(exec taskSQLExecutor) error {
		current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
		if err != nil {
			return err
		}
		if err := requireCurrentRunLease(current, normalized.ClaimToken, normalized.Now); err != nil {
			return err
		}
		leaseUntil := normalized.Now.Add(normalized.LeaseDuration).UTC()
		affected, err := sqlcgen.New(exec).HeartbeatTaskRunLease(ctx, sqlcgen.HeartbeatTaskRunLeaseParams{
			LeaseUntil:     nullableTaskTime(leaseUntil),
			HeartbeatAt:    nullableTaskTime(normalized.Now),
			ClaimToken:     nullableTaskString(normalized.ClaimToken),
			TokensUsed:     normalized.TokensUsed,
			ID:             normalized.RunID,
			ClaimTokenHash: nullableTaskString(current.ClaimTokenHash),
			ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
			StartingStatus: taskpkg.TaskRunStatusStarting.String(),
			RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
		})
		if err != nil {
			return fmt.Errorf("store: heartbeat task run lease %q: %w", normalized.RunID, err)
		}
		if affected == 0 {
			return fmt.Errorf("store: task run lease %q: %w", normalized.RunID, taskpkg.ErrTaskRunNotFound)
		}
		updated, err = g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
		if err != nil {
			return err
		}
		if updated.IsNetworkWake() {
			wakeID, targetSessionID, ownerKey := updated.NetworkWakeCorrelation()
			return appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
				workspaceID: updated.WorkspaceID, wakeID: wakeID, taskRunID: updated.ID,
				ownerKey: ownerKey, targetSessionID: targetSessionID,
				eventType: networkWakeEventHeartbeat, state: updated.Status.String(),
				claimTokenHash: updated.ClaimTokenHash, actor: normalized.Actor.Actor, at: normalized.Now,
			})
		}
		return g.appendLoopTokenTickForHeartbeat(ctx, exec, updated, normalized.TokensUsed)
	}); err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}

func (g *TaskRunRepo) appendLoopTokenTickForHeartbeat(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	tokensUsed int64,
) error {
	loopRunID := strings.TrimSpace(run.LoopRunID)
	if loopRunID == "" || tokensUsed <= 0 {
		return nil
	}
	tokensTotal, err := refreshLoopTokensUsedWithExecutor(ctx, exec, loopRunID)
	if err != nil {
		return err
	}
	if tokensTotal <= 0 {
		return nil
	}
	workspaceID, err := sqlcgen.New(exec).GetLoopRunWorkspaceID(ctx, loopRunID)
	if err != nil {
		return fmt.Errorf("store: load loop run %q workspace for token tick: %w", loopRunID, err)
	}
	return appendLoopTokenTickEventWithExecutor(
		ctx,
		exec,
		loop.RunID(loopRunID),
		loop.WorkspaceID(workspaceID),
		run.ID,
		tokensTotal,
		false,
		run.HeartbeatAt,
	)
}

// ReleaseRunLease clears an active task-run lease after token verification and requeues the run.
func (g *TaskRunRepo) ReleaseRunLease(
	ctx context.Context,
	release taskpkg.LeaseRelease,
) (taskpkg.Run, error) {
	if err := g.checkReady(ctx, "release task run lease"); err != nil {
		return taskpkg.Run{}, err
	}
	normalized, err := release.Normalize(g.now())
	if err != nil {
		return taskpkg.Run{}, err
	}

	var updated taskpkg.Run
	if err := g.tasks.withTaskImmediateTransaction(ctx, "release task run lease", func(exec taskSQLExecutor) error {
		current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
		if err != nil {
			return err
		}
		if err := requireCurrentRunLease(current, normalized.ClaimToken, normalized.Now); err != nil {
			return err
		}
		if err := requeueLeasedRun(ctx, exec, current.ID); err != nil {
			return err
		}
		if current.IsTaskAnchored() {
			if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
				return err
			}
		}
		updated, err = g.tasks.getTaskRunWithExecutor(ctx, exec, current.ID)
		if err != nil {
			return err
		}
		if updated.IsNetworkWake() {
			wakeID, targetSessionID, ownerKey := updated.NetworkWakeCorrelation()
			return appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
				workspaceID: updated.WorkspaceID, wakeID: wakeID, taskRunID: updated.ID,
				ownerKey: ownerKey, targetSessionID: targetSessionID,
				eventType: networkWakeEventReleased, state: updated.Status.String(),
				reason: normalized.Reason, actor: normalized.Actor.Actor, at: normalized.Now,
			})
		}
		return nil
	}); err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}

// CompleteRunLease marks one claimed run complete after token verification.
