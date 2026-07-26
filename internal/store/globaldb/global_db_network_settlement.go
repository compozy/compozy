package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type networkWakeSettlementRow struct {
	wakeID         string
	taskRunID      string
	workspaceID    string
	ownerKey       string
	state          string
	reservedWallMS int64
	actualWallMS   int64
	inputTokens    int64
	outputTokens   int64
	usageState     string
	reason         string
}

// SettleNetworkWake atomically applies one token-fenced Task and Network terminal outcome.
func (g *TaskRunRepo) SettleNetworkWake(
	ctx context.Context,
	settlement taskpkg.NetworkWakeSettlement,
) (taskpkg.NetworkWakeSettlementResult, error) {
	if err := g.checkReady(ctx, "settle network wake"); err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	normalized, err := settlement.Normalize(g.now())
	if err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}

	var result taskpkg.NetworkWakeSettlementResult
	err = g.tasks.withTaskImmediateTransaction(ctx, "settle network wake", func(exec taskSQLExecutor) error {
		var settleErr error
		result, settleErr = g.settleNetworkWakeWithExecutor(ctx, exec, normalized)
		return settleErr
	})
	if err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	return result, nil
}

func (g *TaskRunRepo) settleNetworkWakeWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	settlement taskpkg.NetworkWakeSettlement,
) (taskpkg.NetworkWakeSettlementResult, error) {
	row, err := getNetworkWakeSettlementRow(ctx, exec, settlement.WorkspaceID, settlement.WakeID)
	if err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	run, err := g.tasks.getTaskRunWithExecutor(ctx, exec, row.taskRunID)
	if err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, fmt.Errorf(
			"store: load network wake task run for settlement: %w",
			err,
		)
	}
	if err := requireNetworkWakeSettlementIdentity(run, row, settlement); err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	if row.state != "open" {
		return duplicateNetworkWakeSettlement(run, row, settlement)
	}
	return g.commitNetworkWakeSettlement(ctx, exec, run, row, settlement)
}

func duplicateNetworkWakeSettlement(
	run taskpkg.Run,
	row networkWakeSettlementRow,
	settlement taskpkg.NetworkWakeSettlement,
) (taskpkg.NetworkWakeSettlementResult, error) {
	stored := storedNetworkWakeOutcome(row)
	if !networkWakeSettlementMatches(row, stored, settlement.Outcome) ||
		run.Status.Normalize() != networkWakeTerminalRunStatus(stored.State) {
		return taskpkg.NetworkWakeSettlementResult{}, fmt.Errorf(
			"%w: wake %q already settled as %q",
			taskpkg.ErrNetworkWakeSettlementConflict,
			row.wakeID,
			row.state,
		)
	}
	return taskpkg.NetworkWakeSettlementResult{Run: run, Outcome: stored, Duplicate: true}, nil
}

func (g *TaskRunRepo) commitNetworkWakeSettlement(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	row networkWakeSettlementRow,
	settlement taskpkg.NetworkWakeSettlement,
) (taskpkg.NetworkWakeSettlementResult, error) {
	claimTokenHash := run.ClaimTokenHash
	updated, err := g.terminalizeNetworkWakeRunWithExecutor(ctx, exec, run, settlement)
	if err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	chargedWall, chargedInput, chargedOutput := settledNetworkWakeUsage(row, settlement.Outcome)
	changed, err := compareAndSetNetworkWakeTerminal(
		ctx,
		exec,
		row.workspaceID,
		row.wakeID,
		settlement.Outcome,
		chargedWall,
		chargedInput,
		chargedOutput,
		settlement.Now,
	)
	if err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	if !changed {
		return taskpkg.NetworkWakeSettlementResult{}, fmt.Errorf(
			"%w: wake %q changed during settlement",
			taskpkg.ErrNetworkWakeSettlementConflict,
			row.wakeID,
		)
	}
	if err := settleNetworkWakeBudget(
		ctx,
		exec,
		row,
		run.NetworkSpecSnapshot().Bounds,
		chargedWall,
		chargedInput,
		chargedOutput,
		settlement.Now,
	); err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	if err := appendSettledNetworkWakeEvent(
		ctx,
		exec,
		row,
		settlement,
		claimTokenHash,
		chargedWall,
		chargedInput,
		chargedOutput,
	); err != nil {
		return taskpkg.NetworkWakeSettlementResult{}, err
	}
	return taskpkg.NetworkWakeSettlementResult{Run: updated, Outcome: settlement.Outcome}, nil
}

func appendSettledNetworkWakeEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	row networkWakeSettlementRow,
	settlement taskpkg.NetworkWakeSettlement,
	claimTokenHash string,
	actualWallMS int64,
	inputTokens int64,
	outputTokens int64,
) error {
	return appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
		workspaceID: row.workspaceID, wakeID: row.wakeID, taskRunID: row.taskRunID,
		ownerKey: row.ownerKey, targetSessionID: settlement.TargetSessionID,
		eventType: networkWakeEventSettled, state: settlement.Outcome.State,
		claimTokenHash: claimTokenHash, usageState: settlement.Outcome.UsageState,
		actualWallMS: &actualWallMS, inputTokens: &inputTokens, outputTokens: &outputTokens,
		reason: settlement.Outcome.Reason, actor: settlement.Actor.Actor, at: settlement.Now,
	})
}

func requireNetworkWakeSettlementIdentity(
	run taskpkg.Run,
	row networkWakeSettlementRow,
	settlement taskpkg.NetworkWakeSettlement,
) error {
	wakeID, targetSessionID, ownerKey := run.NetworkWakeCorrelation()
	if !run.IsNetworkWake() || run.ID != settlement.TaskRunID || row.taskRunID != settlement.TaskRunID ||
		run.WorkspaceID != settlement.WorkspaceID || row.workspaceID != settlement.WorkspaceID ||
		wakeID != settlement.WakeID || targetSessionID != settlement.TargetSessionID ||
		ownerKey != settlement.OwnerKey || row.ownerKey != settlement.OwnerKey {
		return fmt.Errorf("%w: network wake settlement identity mismatch", taskpkg.ErrInvalidScopeBinding)
	}
	return nil
}

func (g *TaskRunRepo) terminalizeNetworkWakeRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	settlement taskpkg.NetworkWakeSettlement,
) (taskpkg.Run, error) {
	tokensUsed := settlement.Outcome.ActualInputTokens + settlement.Outcome.ActualOutputTokens
	if settlement.Outcome.State == store.NetworkWakeStateSucceeded {
		payload, err := json.Marshal(map[string]string{"network_wake_id": settlement.WakeID})
		if err != nil {
			return taskpkg.Run{}, fmt.Errorf("store: encode network wake result: %w", err)
		}
		completion, err := (taskpkg.LeaseCompletion{
			RunID: run.ID, ClaimToken: settlement.ClaimToken,
			Result:     taskpkg.RunResult{Value: payload, TokensUsed: tokensUsed},
			TokensUsed: tokensUsed, Now: settlement.Now, Actor: settlement.Actor,
		}).Normalize(settlement.Now)
		if err != nil {
			return taskpkg.Run{}, err
		}
		return g.completeRunLeaseWithExecutor(ctx, exec, completion)
	}
	failure, err := (taskpkg.LeaseFailure{
		RunID: run.ID, ClaimToken: settlement.ClaimToken,
		Failure:    taskpkg.RunFailure{Error: settlement.Outcome.Reason},
		TokensUsed: tokensUsed, Now: settlement.Now, Actor: settlement.Actor,
	}).Normalize(settlement.Now)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return g.failRunLeaseWithExecutor(ctx, exec, failure)
}

func getNetworkWakeSettlementRow(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID string,
	wakeID string,
) (networkWakeSettlementRow, error) {
	// dynamic-sql: settlement locks this accounting row through the enclosing immediate transaction.
	const query = `
SELECT wake_id, task_run_id, workspace_id, owner_key, state, reserved_wall_ms,
       COALESCE(actual_wall_ms, 0), COALESCE(input_tokens, 0), COALESCE(output_tokens, 0),
       usage_state, reason
FROM network_live_wakes
WHERE workspace_id = ? AND wake_id = ?`
	var row networkWakeSettlementRow
	if err := exec.QueryRowContext(ctx, query, workspaceID, wakeID).Scan(
		&row.wakeID,
		&row.taskRunID,
		&row.workspaceID,
		&row.ownerKey,
		&row.state,
		&row.reservedWallMS,
		&row.actualWallMS,
		&row.inputTokens,
		&row.outputTokens,
		&row.usageState,
		&row.reason,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return networkWakeSettlementRow{}, fmt.Errorf("store: network wake %q not found", wakeID)
		}
		return networkWakeSettlementRow{}, fmt.Errorf("store: get network wake settlement row: %w", err)
	}
	return row, nil
}

func storedNetworkWakeOutcome(row networkWakeSettlementRow) store.NetworkWakeOutcome {
	outcome := store.NetworkWakeOutcome{
		State:          row.state,
		ActualWallTime: time.Duration(row.actualWallMS) * time.Millisecond,
		UsageState:     row.usageState,
		Reason:         row.reason,
	}
	if row.usageState == store.NetworkWakeUsageActual {
		outcome.ActualInputTokens = row.inputTokens
		outcome.ActualOutputTokens = row.outputTokens
	}
	return outcome
}

func networkWakeSettlementMatches(
	row networkWakeSettlementRow,
	stored store.NetworkWakeOutcome,
	requested store.NetworkWakeOutcome,
) bool {
	wallMS, inputTokens, outputTokens := settledNetworkWakeUsage(row, requested)
	return stored.State == requested.State && stored.UsageState == requested.UsageState &&
		stored.Reason == requested.Reason && row.actualWallMS == wallMS &&
		row.inputTokens == inputTokens && row.outputTokens == outputTokens
}

func networkWakeTerminalRunStatus(state string) taskpkg.RunStatus {
	if state == store.NetworkWakeStateSucceeded {
		return taskpkg.TaskRunStatusCompleted
	}
	return taskpkg.TaskRunStatusFailed
}

func settledNetworkWakeUsage(
	row networkWakeSettlementRow,
	outcome store.NetworkWakeOutcome,
) (wallMS int64, inputTokens int64, outputTokens int64) {
	wallMS = durationMillisecondsCeil(outcome.ActualWallTime)
	if wallMS == 0 && outcome.UsageState == store.NetworkWakeUsageUnavailable {
		wallMS = row.reservedWallMS
	}
	if outcome.UsageState == store.NetworkWakeUsageUnavailable {
		return wallMS, row.inputTokens, row.outputTokens
	}
	return wallMS, outcome.ActualInputTokens, outcome.ActualOutputTokens
}

func compareAndSetNetworkWakeTerminal(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID string,
	wakeID string,
	outcome store.NetworkWakeOutcome,
	wallMS int64,
	inputTokens int64,
	outputTokens int64,
	settledAt time.Time,
) (bool, error) {
	// dynamic-sql: state='open' is the settlement idempotency fence.
	const query = `
UPDATE network_live_wakes
SET state = ?, actual_wall_ms = ?, settled_at = ?, input_tokens = ?,
    output_tokens = ?, usage_state = ?, reason = ?
WHERE workspace_id = ? AND wake_id = ? AND state = 'open'`
	result, err := exec.ExecContext(
		ctx,
		query,
		normalizeNetworkWakeState(outcome.State),
		wallMS,
		store.FormatTimestamp(settledAt),
		inputTokens,
		outputTokens,
		strings.TrimSpace(outcome.UsageState),
		strings.TrimSpace(outcome.Reason),
		workspaceID,
		wakeID,
	)
	if err != nil {
		return false, fmt.Errorf("store: settle network wake ledger: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: inspect network wake settlement: %w", err)
	}
	return rowsAffected == 1, nil
}

func settleNetworkWakeBudget(
	ctx context.Context,
	exec taskSQLExecutor,
	row networkWakeSettlementRow,
	bounds participation.Bounds,
	wallMS int64,
	inputTokens int64,
	outputTokens int64,
	settledAt time.Time,
) error {
	budget, err := getNetworkWakeBudget(ctx, exec, row.workspaceID, row.ownerKey)
	if err != nil {
		return err
	}
	next := networkWakeBudget{
		wakesUsed:        budget.wakesUsed,
		wallMSUsed:       budget.wallMSUsed - row.reservedWallMS + wallMS,
		inputTokensUsed:  budget.inputTokensUsed - row.inputTokens + inputTokens,
		outputTokensUsed: budget.outputTokensUsed - row.outputTokens + outputTokens,
	}
	reason, err := settledNetworkWakeBudgetReason(next, bounds)
	if err != nil {
		return err
	}
	// dynamic-sql: this update is paired with Task and wake terminal CASes above.
	const query = `
UPDATE network_participation_budgets
SET wall_ms_used = ?, input_tokens_used = ?, output_tokens_used = ?,
    exhausted_reason = ?, updated_at = ?
WHERE workspace_id = ? AND owner_key = ?`
	result, err := exec.ExecContext(
		ctx,
		query,
		next.wallMSUsed,
		next.inputTokensUsed,
		next.outputTokensUsed,
		reason,
		store.FormatTimestamp(settledAt),
		row.workspaceID,
		row.ownerKey,
	)
	if err != nil {
		return fmt.Errorf("store: settle network participation budget: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect network participation budget settlement: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("store: network participation budget %q not found", row.ownerKey)
	}
	return nil
}

func settledNetworkWakeBudgetReason(
	budget networkWakeBudget,
	bounds participation.Bounds,
) (string, error) {
	totalWall, err := time.ParseDuration(bounds.MaxTotalWallTime)
	if err != nil {
		return "", fmt.Errorf("store: parse settled network total wall time: %w", err)
	}
	switch {
	case budget.wakesUsed >= bounds.MaxWakes:
		return networkWakeExhaustionMaxWakes, nil
	case budget.wallMSUsed >= durationMillisecondsCeil(totalWall):
		return networkWakeExhaustionMaxTotalWallTime, nil
	case budget.inputTokensUsed >= bounds.MaxInputTokens:
		return networkWakeExhaustionMaxInputTokens, nil
	case budget.outputTokensUsed >= bounds.MaxOutputTokens:
		return networkWakeExhaustionMaxOutputTokens, nil
	default:
		return "", nil
	}
}
