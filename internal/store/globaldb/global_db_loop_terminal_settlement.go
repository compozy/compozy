package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	loopRunTerminalReason       = "loop_run_terminal"
	reconciledRunTerminalReason = "reconciled_run_terminal"
	runMissingReason            = "run_missing"
	loopSettlementActorRef      = "loop-reconciler"
)

type loopSettlementRecord struct {
	taskID      string
	runID       string
	status      taskpkg.Status
	coordinator bool
}

type loopSettlementOutcome struct {
	result         looppkg.SettleResult
	recordsSettled int
}

type loopSettlementRunRepair struct {
	taskID string
	runID  string
}

// settleLoopRunTerminal is the sole authority for terminal Loop execution-record settlement.
func settleLoopRunTerminal(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	cause looppkg.TerminalCause,
) (looppkg.SettleResult, error) {
	outcome, err := settleLoopRunTerminalWithReason(ctx, exec, runID, cause, loopRunTerminalReason)
	return outcome.result, err
}

func settleLoopRunTerminalWithReason(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	cause looppkg.TerminalCause,
	reason string,
) (loopSettlementOutcome, error) {
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return loopSettlementOutcome{}, fmt.Errorf("%w: loop run id is required", looppkg.ErrValidation)
	}
	coordinatorStatus, detail, err := loopSettlementTarget(cause)
	if err != nil {
		return loopSettlementOutcome{}, err
	}
	records, err := listLoopSettlementRecords(ctx, exec, trimmedRunID)
	if err != nil {
		return loopSettlementOutcome{}, err
	}
	return settleLoopRunTerminalRecordsWithReason(
		ctx, exec, trimmedRunID, coordinatorStatus, detail, reason, records,
	)
}

func settleLoopRunTerminalWithRecords(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	cause looppkg.TerminalCause,
	records []loopSettlementRecord,
) (looppkg.SettleResult, error) {
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return looppkg.SettleResult{}, fmt.Errorf("%w: loop run id is required", looppkg.ErrValidation)
	}
	coordinatorStatus, detail, err := loopSettlementTarget(cause)
	if err != nil {
		return looppkg.SettleResult{}, err
	}
	outcome, err := settleLoopRunTerminalRecordsWithReason(
		ctx, exec, trimmedRunID, coordinatorStatus, detail, loopRunTerminalReason, records,
	)
	return outcome.result, err
}

func settleLoopRunTerminalRecordsWithReason(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	coordinatorStatus taskpkg.Status,
	detail string,
	reason string,
	records []loopSettlementRecord,
) (loopSettlementOutcome, error) {
	actor, err := taskpkg.DeriveDaemonActorContext(loopSettlementActorRef, loopSettlementActorRef)
	if err != nil {
		return loopSettlementOutcome{}, fmt.Errorf("store: derive Loop settlement actor: %w", err)
	}
	now := time.Now().UTC()
	runRepairs, err := cancelLoopSettlementRuns(ctx, exec, runID, now)
	if err != nil {
		return loopSettlementOutcome{}, err
	}
	outcome := loopSettlementOutcome{result: looppkg.SettleResult{
		RunsCanceled: len(runRepairs), CoordinatorStatus: coordinatorStatus,
	}}
	recordStatus := make(map[string]taskpkg.Status, len(records))
	for _, record := range records {
		recordStatus[record.taskID] = record.status
	}
	for _, repair := range runRepairs {
		status, ok := recordStatus[repair.taskID]
		if !ok || !loopTaskStatusTerminal(status) {
			continue
		}
		if err := appendTaskStatusChangedAuditEvent(ctx, exec, repair.taskID, status, status, actor, now,
			taskStatusEventContext{
				reason: reason, detail: detail, runID: repair.runID,
				loopRunID: runID, releaseReason: reason,
			}); err != nil {
			return loopSettlementOutcome{}, err
		}
		outcome.recordsSettled++
	}
	for _, record := range records {
		target := taskpkg.TaskStatusCanceled
		if record.coordinator {
			target = coordinatorStatus
		}
		if loopTaskStatusTerminal(record.status) {
			continue
		}
		if err := settleLoopTaskRecord(
			ctx, exec, record, target, actor, now, runID, reason, detail,
		); err != nil {
			return loopSettlementOutcome{}, err
		}
		outcome.recordsSettled++
		if !record.coordinator {
			outcome.result.CellsSettled++
		}
	}
	return outcome, nil
}

func loopTaskStatusTerminal(status taskpkg.Status) bool {
	switch status.Normalize() {
	case taskpkg.TaskStatusCompleted, taskpkg.TaskStatusFailed, taskpkg.TaskStatusCanceled:
		return true
	default:
		return false
	}
}

func loopSettlementTarget(cause looppkg.TerminalCause) (taskpkg.Status, string, error) {
	switch cause {
	case looppkg.TerminalCauseDone, looppkg.TerminalCauseNoOp:
		return taskpkg.TaskStatusCompleted, "run done; node no longer needed", nil
	case looppkg.TerminalCauseFailed, looppkg.TerminalCauseExhausted, looppkg.TerminalCauseStalled:
		return taskpkg.TaskStatusFailed, "run " + string(cause) + "; node no longer needed", nil
	case looppkg.TerminalCauseCanceled, looppkg.TerminalCauseKilled:
		return taskpkg.TaskStatusCanceled, "run " + string(cause) + "; node no longer needed", nil
	case looppkg.TerminalCauseRunMissing:
		return taskpkg.TaskStatusCanceled, runMissingReason, nil
	default:
		return "", "", fmt.Errorf("%w: unsupported terminal cause %q", looppkg.ErrValidation, cause)
	}
}

func listLoopSettlementRecords(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRunID string,
) (records []loopSettlementRecord, err error) {
	rows, err := exec.QueryContext(ctx, `WITH RECURSIVE coordinator(task_id) AS (
		SELECT task_id FROM task_runs WHERE loop_run_id = ? AND run_kind = 'coordinator' LIMIT 1
	), descendants(task_id, depth) AS (
		SELECT id, 1 FROM tasks WHERE parent_task_id = (SELECT task_id FROM coordinator)
		UNION ALL
		SELECT child.id, parent.depth + 1
		FROM tasks child JOIN descendants parent ON child.parent_task_id = parent.task_id
	), owned(task_id, depth) AS (
		SELECT task_id, 0 FROM task_runs WHERE loop_run_id = ? AND task_id IS NOT NULL
		UNION ALL SELECT task_id, depth FROM descendants
	), settlement(task_id, depth) AS (
		SELECT task_id, MAX(depth) FROM owned GROUP BY task_id
	)
	SELECT t.id, COALESCE((SELECT tr.id FROM task_runs tr
		WHERE tr.task_id = t.id AND tr.loop_run_id = ? ORDER BY tr.queued_at DESC LIMIT 1), ''),
		t.status, t.id = COALESCE((SELECT task_id FROM coordinator), '')
	FROM tasks t JOIN settlement s ON s.task_id = t.id
	ORDER BY t.id = COALESCE((SELECT task_id FROM coordinator), ''), s.depth DESC, t.id`,
		loopRunID, loopRunID, loopRunID)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop settlement records for %q: %w", loopRunID, err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "Loop settlement records") }()
	for rows.Next() {
		var record loopSettlementRecord
		if scanErr := rows.Scan(
			&record.taskID, &record.runID, &record.status, &record.coordinator,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan Loop settlement record: %w", scanErr)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate Loop settlement records: %w", err)
	}
	return records, nil
}

func cancelLoopSettlementRuns(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRunID string,
	now time.Time,
) ([]loopSettlementRunRepair, error) {
	repairs, err := listLoopSettlementRunRepairs(ctx, exec, loopRunID)
	if err != nil {
		return nil, err
	}

	result, err := exec.ExecContext(ctx, `UPDATE task_runs
		SET status = 'canceled', ended_at = ?, error = 'Loop run reached a terminal state',
			claim_token = NULL, lease_until = NULL, heartbeat_at = NULL
		WHERE loop_run_id = ? AND status IN ('queued','claimed','starting','running','needs_attention')`,
		store.FormatTimestamp(now), loopRunID)
	if err != nil {
		return nil, fmt.Errorf("store: cancel live task runs for Loop %q: %w", loopRunID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("store: count canceled task runs for Loop %q: %w", loopRunID, err)
	}
	if affected != int64(len(repairs)) {
		return nil, fmt.Errorf(
			"store: canceled task runs for Loop %q = %d, want %d", loopRunID, affected, len(repairs),
		)
	}
	for _, repair := range repairs {
		run, err := (&TaskRepo{}).getTaskRunWithExecutor(ctx, exec, repair.runID)
		if err != nil {
			return nil, fmt.Errorf("store: load terminal Loop task run %q: %w", repair.runID, err)
		}
		if err := closeTerminalRunAgentBinding(ctx, exec, run, now); err != nil {
			return nil, fmt.Errorf("store: close terminal Loop task run %q binding: %w", repair.runID, err)
		}
	}
	if _, err := exec.ExecContext(ctx, `UPDATE tasks SET current_run_id = NULL
		WHERE current_run_id IN (SELECT id FROM task_runs WHERE loop_run_id = ?
		AND status = 'canceled')`, loopRunID); err != nil {
		return nil, fmt.Errorf("store: clear current runs for Loop %q: %w", loopRunID, err)
	}
	return repairs, nil
}

func listLoopSettlementRunRepairs(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRunID string,
) (repairs []loopSettlementRunRepair, err error) {
	rows, err := exec.QueryContext(ctx, `SELECT id, task_id FROM task_runs
		WHERE loop_run_id = ? AND status IN ('queued','claimed','starting','running','needs_attention')
		ORDER BY id`, loopRunID)
	if err != nil {
		return nil, fmt.Errorf("store: list live task runs for Loop %q: %w", loopRunID, err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "live Loop task runs") }()
	for rows.Next() {
		var repair loopSettlementRunRepair
		if scanErr := rows.Scan(&repair.runID, &repair.taskID); scanErr != nil {
			return nil, fmt.Errorf("store: scan live task run for Loop %q: %w", loopRunID, scanErr)
		}
		repairs = append(repairs, repair)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate live task runs for Loop %q: %w", loopRunID, err)
	}
	return repairs, nil
}

func settleLoopTaskRecord(
	ctx context.Context,
	exec taskSQLExecutor,
	record loopSettlementRecord,
	target taskpkg.Status,
	actor taskpkg.ActorContext,
	now time.Time,
	loopRunID string,
	reason string,
	detail string,
) error {
	if _, err := exec.ExecContext(ctx, `UPDATE tasks SET updated_at = ?, closed_at = ?,
		needs_attention_reason = NULL, needs_attention_at = NULL,
		needs_attention_by_kind = NULL, needs_attention_by_ref = NULL WHERE id = ?`,
		store.FormatTimestamp(now), store.FormatTimestamp(now), record.taskID); err != nil {
		return fmt.Errorf("store: prepare Loop task %q settlement: %w", record.taskID, err)
	}
	eventRunID := record.runID
	if eventRunID != "" {
		var owner string
		if err := exec.QueryRowContext(ctx, `SELECT task_id FROM task_runs WHERE id = ?`, eventRunID).Scan(&owner); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("store: validate Loop settlement run %q: %w", eventRunID, err)
			}
			eventRunID = ""
		}
	}
	if err := setTaskStatusWithEventContext(ctx, exec, record.taskID, record.status, target, actor, now,
		taskStatusEventContext{
			reason: reason, detail: detail, runID: eventRunID, loopRunID: loopRunID, releaseReason: reason,
		}); err != nil {
		return err
	}
	return nil
}

func loopSettlementTransitionsForRecords(
	ctx context.Context,
	exec taskSQLExecutor,
	records []loopSettlementRecord,
) ([]taskpkg.StatusTransition, error) {
	transitions := make([]taskpkg.StatusTransition, 0, len(records))
	for _, record := range records {
		if loopTaskStatusTerminal(record.status) {
			continue
		}
		updated, err := (&TaskRepo{}).getTaskWithExecutor(ctx, exec, record.taskID)
		if err != nil {
			return nil, err
		}
		transitions = append(transitions, taskpkg.StatusTransition{
			Task: updated, PreviousStatus: record.status,
		})
	}
	return transitions, nil
}

func terminalCauseForLoopStatus(
	status looppkg.Status,
	cause looppkg.TransitionCause,
) (looppkg.TerminalCause, error) {
	if status == looppkg.StatusCanceled && cause == looppkg.TransitionCauseOperatorKill {
		return looppkg.TerminalCauseKilled, nil
	}
	switch status {
	case looppkg.StatusDone:
		return looppkg.TerminalCauseDone, nil
	case looppkg.StatusNoOp:
		return looppkg.TerminalCauseNoOp, nil
	case looppkg.StatusBlocked, looppkg.StatusFailed:
		return looppkg.TerminalCauseFailed, nil
	case looppkg.StatusExhausted:
		return looppkg.TerminalCauseExhausted, nil
	case looppkg.StatusStalled:
		return looppkg.TerminalCauseStalled, nil
	case looppkg.StatusCanceled:
		return looppkg.TerminalCauseCanceled, nil
	default:
		return "", fmt.Errorf("%w: unsupported terminal loop status %q", looppkg.ErrValidation, status)
	}
}
