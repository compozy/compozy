package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	loop "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	globalDBTaskClaimStatusKey  = "status"
	globalDBTaskClaimRequestKey = "request"
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
	if criteria.RunKind.Normalize() == taskpkg.RunKindCallActivation {
		if err := markCallRunningForActivation(ctx, exec, runID, criteria.Now); err != nil {
			return taskpkg.ClaimResult{}, err
		}
	}
	run, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	if run.IsNetworkWake() {
		return claimNetworkWakeResult(ctx, exec, run, claimToken, leaseUntil, criteria.Now)
	}
	if run.IsCallActivation() {
		return taskpkg.ClaimResult{Run: run, ClaimToken: claimToken, LeaseUntil: leaseUntil}, nil
	}
	return g.claimStandardTaskRunResult(ctx, exec, run, claimToken, leaseUntil, criteria.Now)
}

func markCallRunningForActivation(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	startedAt time.Time,
) error {
	result, err := exec.ExecContext(ctx, `UPDATE calls
		SET state = 'running', started_at = ?, updated_at = ?
		WHERE activation_run_id = ? AND state = 'queued'`,
		store.FormatTimestamp(startedAt),
		store.FormatTimestamp(startedAt),
		strings.TrimSpace(runID),
	)
	if err != nil {
		return fmt.Errorf("store: mark call activation %q running: %w", runID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect call activation %q running update: %w", runID, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: call activation run %q lost its queued call", taskpkg.ErrNoClaimableRun, runID)
	}
	return nil
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
