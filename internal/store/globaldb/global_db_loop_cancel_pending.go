package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ looppkg.CancellationReconciliationStore = (*LoopRepo)(nil)

// ListPendingCancellations returns a bounded, workspace-scoped delivery batch.
func (g *LoopRepo) ListPendingCancellations(
	ctx context.Context,
	limit int,
) (pending []looppkg.PendingCancellation, err error) {
	if err := g.checkReady(ctx, "list pending Loop cancellations"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: pending cancellation limit must be positive", looppkg.ErrValidation)
	}

	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("store: begin pending Loop cancellations read: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf(
				"store: rollback pending Loop cancellations read: %w",
				rollbackErr,
			))
		}
	}()

	pending, err = queryPendingCancellations(ctx, tx, limit)
	if err != nil {
		return nil, err
	}
	if err := hydratePendingCancellationSessions(ctx, tx, pending); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: commit pending Loop cancellations read: %w", err)
	}
	committed = true
	return pending, nil
}

func queryPendingCancellations(
	ctx context.Context,
	exec taskSQLExecutor,
	limit int,
) (pending []looppkg.PendingCancellation, err error) {
	rows, err := exec.QueryContext(ctx, `SELECT workspace_id, run_id, node_id, cancel_state,
		cancel_reason, cancel_requested_at, cancel_actor_kind, cancel_actor_id
	FROM (
		SELECT run.workspace_id AS workspace_id, run.id AS run_id, '' AS node_id,
			CASE WHEN MAX(control.cancel_state = 'requested') = 1
				THEN 'requested' ELSE 'delivering' END AS cancel_state,
			COALESCE(MIN(control.cancel_reason), '') AS cancel_reason,
			MIN(control.cancel_requested_at) AS cancel_requested_at,
			COALESCE(MIN(control.cancel_actor_kind), '') AS cancel_actor_kind,
			COALESCE(MIN(control.cancel_actor_id), '') AS cancel_actor_id
		FROM loop_runs AS run
		JOIN loop_node_controls AS control ON control.loop_run_id = run.id
		WHERE run.cancel_requested = 1 AND run.cancel_kind = 'cancel'
			AND run.status IN ('queued','running','watching','needs-approval','paused')
			AND control.cancel_state IN ('requested','delivering')
		GROUP BY run.workspace_id, run.id
		UNION ALL
		SELECT run.workspace_id, run.id, control.node_id, control.cancel_state,
			COALESCE(control.cancel_reason, ''), control.cancel_requested_at,
			COALESCE(control.cancel_actor_kind, ''), COALESCE(control.cancel_actor_id, '')
		FROM loop_node_controls AS control
		JOIN loop_runs AS run ON run.id = control.loop_run_id
		WHERE run.cancel_requested = 0
			AND run.status IN ('queued','running','watching','needs-approval','paused')
			AND control.cancel_state IN ('requested','delivering')
	)
	ORDER BY cancel_requested_at, run_id, node_id
	LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: query pending Loop cancellations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close pending Loop cancellations: %w", closeErr))
		}
	}()

	for rows.Next() {
		command, scanErr := scanPendingCancellation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		pending = append(pending, command)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate pending Loop cancellations: %w", err)
	}
	return pending, nil
}

func scanPendingCancellation(rows *sql.Rows) (looppkg.PendingCancellation, error) {
	var command looppkg.PendingCancellation
	var workspaceID, runID, nodeID, state, actorKind, actorID string
	var requestedAt sql.NullTime
	if err := rows.Scan(
		&workspaceID,
		&runID,
		&nodeID,
		&state,
		&command.Reason,
		&requestedAt,
		&actorKind,
		&actorID,
	); err != nil {
		return looppkg.PendingCancellation{}, fmt.Errorf("store: scan pending Loop cancellation: %w", err)
	}
	if !requestedAt.Valid {
		return looppkg.PendingCancellation{}, errors.New("store: pending Loop cancellation requested_at is missing")
	}
	command.WorkspaceID = looppkg.WorkspaceID(workspaceID)
	command.RunID = looppkg.RunID(runID)
	command.NodeID = looppkg.NodeID(nodeID)
	command.State = looppkg.CancelState(state)
	command.RequestedAt = requestedAt.Time.UTC()
	command.RequestedBy = taskpkg.ActorIdentity{Kind: taskpkg.ActorKind(actorKind), Ref: actorID}
	return command, nil
}

func hydratePendingCancellationSessions(
	ctx context.Context,
	exec taskSQLExecutor,
	pending []looppkg.PendingCancellation,
) error {
	for index := range pending {
		mutation := looppkg.CancellationMutation{
			WorkspaceID: pending[index].WorkspaceID,
			RunID:       pending[index].RunID,
			NodeID:      pending[index].NodeID,
			Kind:        looppkg.RunCancelCancel,
		}
		var err error
		if pending[index].NodeID == "" {
			pending[index].SessionIDs, err = listRunCancellationSessions(ctx, exec, mutation)
		} else {
			pending[index].SessionIDs, err = listNodeCancellationSessions(ctx, exec, mutation)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
