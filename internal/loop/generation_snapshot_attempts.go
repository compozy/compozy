package loop

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/internal/task"
)

func writeGenerationAttempt(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	generation int,
	attempt NodeAttempt,
) error {
	attempt = attempt.normalized(RunID(loopRunID))
	if err := attempt.validate(RunID(loopRunID), generation); err != nil {
		return err
	}
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO loop_node_attempts (
			loop_run_id, generation, node_id, item_index, attempt,
			failure_class, failure_code, cause, hint, target, disposition,
			started_at, ended_at, next_attempt_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(loop_run_id, generation, node_id, item_index, attempt) DO NOTHING`,
		loopRunID,
		generation,
		attempt.NodeID,
		attempt.ItemIndex,
		attempt.Attempt,
		failureClassSQLValue(attempt.FailureClass),
		attempt.FailureCode,
		attempt.Cause,
		attempt.Hint,
		attempt.Target,
		attempt.Disposition,
		attempt.StartedAt,
		attempt.EndedAt,
		attempt.NextAttemptAt,
	)
	if err != nil {
		return fmt.Errorf(
			"loop: write node attempt %s/%d/a%d: %w",
			attempt.NodeID,
			attempt.ItemIndex,
			attempt.Attempt,
			err,
		)
	}
	if _, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("loop: inspect node attempt write: %w", err)
	}
	return nil
}

func failureClassSQLValue(value *FailureClass) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*value), Valid: true}
}
