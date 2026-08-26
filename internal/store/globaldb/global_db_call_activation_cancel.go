package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ taskpkg.ActivationRunCancelStore = (*TaskRunRepo)(nil)

func (g *TaskRunRepo) CancelCallActivationRun(
	ctx context.Context,
	runID string,
	reason string,
	at time.Time,
) (outcome taskpkg.ActivationCancelOutcome, err error) {
	if err := g.checkReady(ctx, "cancel call activation run"); err != nil {
		return taskpkg.ActivationCancelOutcome{}, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "cancel call activation run", func(exec taskSQLExecutor) error {
		var status string
		err := exec.QueryRowContext(ctx,
			`SELECT status FROM task_runs WHERE id = ? AND run_kind = 'call_activation'`,
			strings.TrimSpace(runID),
		).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("store: inspect call activation run %q: %w", runID, err)
		}
		switch status {
		case taskpkg.TaskRunStatusQueued.String():
			result, err := exec.ExecContext(ctx, `UPDATE task_runs SET status = 'canceled', error = ?,
				ended_at = ?, claim_token = NULL, claim_token_hash = NULL, lease_until = NULL,
				heartbeat_at = NULL WHERE id = ? AND status = 'queued'`,
				nullableTaskString(reason), store.FormatTimestamp(at), strings.TrimSpace(runID))
			if err != nil {
				return fmt.Errorf("store: cancel queued call activation run %q: %w", runID, err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("store: inspect canceled call activation run %q: %w", runID, err)
			}
			outcome.Won = affected == 1
		case taskpkg.TaskRunStatusClaimed.String(),
			taskpkg.TaskRunStatusStarting.String(),
			taskpkg.TaskRunStatusRunning.String():
			outcome.Claimed = true
			result, err := exec.ExecContext(ctx, `UPDATE task_runs SET status = 'canceled', error = ?,
				ended_at = ?, claim_token = NULL, claim_token_hash = NULL, lease_until = NULL,
				heartbeat_at = NULL WHERE id = ? AND status IN ('claimed', 'starting', 'running')`,
				nullableTaskString(reason), store.FormatTimestamp(at), strings.TrimSpace(runID))
			if err != nil {
				return fmt.Errorf("store: cancel active call activation run %q: %w", runID, err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("store: inspect active call activation cancellation %q: %w", runID, err)
			}
			outcome.Won = affected == 1
		}
		return nil
	})
	return outcome, err
}
