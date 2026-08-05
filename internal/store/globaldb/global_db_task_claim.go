package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	loop "github.com/compozy/compozy/internal/loop"
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
