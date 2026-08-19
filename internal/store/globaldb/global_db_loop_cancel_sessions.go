package globaldb

import (
	"context"
	"errors"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func listRunCancellationSessions(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) ([]string, error) {
	rows, err := exec.QueryContext(ctx, `SELECT DISTINCT session_id FROM loop_session_bindings
		WHERE loop_run_id = ? AND workspace_id = ? AND state = 'active' ORDER BY session_id`,
		mutation.RunID, mutation.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop cancellation sessions: %w", err)
	}
	return scanCancellationSessions(rows)
}

func listNodeCancellationSessions(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) ([]string, error) {
	query := `SELECT DISTINCT binding.session_id
		FROM loop_session_bindings AS binding
		JOIN loop_generation_outputs AS output ON output.loop_run_id = binding.loop_run_id
		JOIN task_runs AS task_run ON task_run.id = output.task_run_id
		WHERE binding.loop_run_id = ? AND binding.workspace_id = ? AND binding.state = 'active'
		  AND output.node_id = ? AND output.status IN (` + liveCancelOutputStatuses + `)
		  AND json_extract(task_run.metadata_json, '$.session_handle') = binding.handle`
	args := []any{mutation.RunID, mutation.WorkspaceID, mutation.NodeID}
	if mutation.ItemIndex != nil {
		query += ` AND output.item_index = ?`
		args = append(args, *mutation.ItemIndex)
	}
	query += ` ORDER BY binding.session_id`
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop node cancellation sessions: %w", err)
	}
	return scanCancellationSessions(rows)
}

type cancellationSessionRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func scanCancellationSessions(rows cancellationSessionRows) ([]string, error) {
	var sessions []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, errors.Join(
				fmt.Errorf("store: scan Loop cancellation session: %w", err),
				rows.Close(),
			)
		}
		sessions = append(sessions, sessionID)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("store: iterate Loop cancellation sessions: %w", err),
			rows.Close(),
		)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("store: close Loop cancellation sessions: %w", err)
	}
	return sessions, nil
}
