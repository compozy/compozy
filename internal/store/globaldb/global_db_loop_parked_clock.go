package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

type loopNodeScheduleAnchor struct {
	itemIndex int
	at        time.Time
}

func shiftLoopWallClockIfUnparked(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	parkedFor time.Duration,
) error {
	if parkedFor <= 0 {
		return nil
	}
	var parked int
	err := exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_generation_outputs AS output
		JOIN loop_runs AS run ON run.id = output.loop_run_id AND run.generation = output.generation
		LEFT JOIN loop_node_controls AS control ON control.loop_run_id = output.loop_run_id
		 AND control.node_id = output.node_id
		WHERE output.loop_run_id = ? AND (output.status IN ('paused','waiting','awaiting_goal')
		 OR (output.status = 'quarantined' AND COALESCE(control.quarantined, 0) = 1))`,
		runID).Scan(&parked)
	if err != nil {
		return fmt.Errorf("store: inspect Loop parked wall clock: %w", err)
	}
	if parked == 0 {
		err = exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_node_waits
			WHERE loop_run_id = ? AND kind = 'approval_escalation'
			AND claim_state IN ('waiting','intervention_required')`, runID).Scan(&parked)
		if err != nil {
			return fmt.Errorf("store: inspect Loop approval wall clock: %w", err)
		}
	}
	if parked > 0 {
		return nil
	}
	var startedAtRaw string
	if err := exec.QueryRowContext(ctx, `SELECT started_at FROM loop_runs WHERE id = ?`, runID).
		Scan(&startedAtRaw); err != nil {
		return fmt.Errorf("store: load Loop wall-clock anchor: %w", err)
	}
	startedAt, err := parseLoopRunTimestamp(startedAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse Loop wall-clock anchor: %w", err)
	}
	_, err = exec.ExecContext(ctx, `UPDATE loop_runs SET started_at = ? WHERE id = ?`,
		startedAt.UTC().Add(parkedFor).Format(time.RFC3339Nano), runID)
	if err != nil {
		return fmt.Errorf("store: shift Loop wall clock after parked interval: %w", err)
	}
	return nil
}

func shiftPausedNodeFirstScheduledAnchors(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
	parkedFor time.Duration,
) error {
	return shiftParkedNodeFirstScheduledAnchors(ctx, exec, runID, nodeID, "paused", parkedFor)
}

func shiftQuarantinedNodeFirstScheduledAnchors(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
	parkedFor time.Duration,
) error {
	return shiftParkedNodeFirstScheduledAnchors(ctx, exec, runID, nodeID, "quarantined", parkedFor)
}

func shiftParkedNodeFirstScheduledAnchors(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
	status string,
	parkedFor time.Duration,
) (returnErr error) {
	if parkedFor <= 0 {
		return nil
	}
	rows, err := exec.QueryContext(ctx, `SELECT item_index, first_scheduled_at
		FROM loop_generation_outputs WHERE loop_run_id = ?
		AND generation = (SELECT generation FROM loop_runs WHERE id = ?)
		AND node_id = ? AND status = ? AND first_scheduled_at IS NOT NULL
		ORDER BY item_index`, runID, runID, nodeID, status)
	if err != nil {
		return fmt.Errorf("store: list parked Loop node clocks: %w", err)
	}
	anchors := make([]loopNodeScheduleAnchor, 0)
	for rows.Next() {
		var itemIndex int
		var raw string
		if err := rows.Scan(&itemIndex, &raw); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				return errors.Join(fmt.Errorf("store: scan parked Loop node clock: %w", err), closeErr)
			}
			return fmt.Errorf("store: scan parked Loop node clock: %w", err)
		}
		parsed, err := parseLoopRunTimestamp(raw)
		if err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				return errors.Join(fmt.Errorf("store: parse parked Loop node clock: %w", err), closeErr)
			}
			return fmt.Errorf("store: parse parked Loop node clock: %w", err)
		}
		anchors = append(anchors, loopNodeScheduleAnchor{itemIndex: itemIndex, at: parsed})
	}
	if err := rows.Err(); err != nil {
		if closeErr := rows.Close(); closeErr != nil {
			return errors.Join(fmt.Errorf("store: iterate parked Loop node clocks: %w", err), closeErr)
		}
		return fmt.Errorf("store: iterate parked Loop node clocks: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("store: close parked Loop node clocks: %w", err)
	}
	for _, anchor := range anchors {
		if _, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET first_scheduled_at = ?
			WHERE loop_run_id = ? AND generation = (SELECT generation FROM loop_runs WHERE id = ?)
			AND node_id = ? AND item_index = ? AND status = ?`,
			anchor.at.UTC().Add(parkedFor).Format(time.RFC3339Nano), runID, runID, nodeID,
			anchor.itemIndex, status); err != nil {
			return fmt.Errorf("store: shift parked Loop node clock: %w", err)
		}
	}
	return nil
}

func shiftedLoopCellFirstScheduledAt(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	generation int,
	nodeID looppkg.NodeID,
	itemIndex int,
	parkedFor time.Duration,
) (any, error) {
	var raw sql.NullString
	if err := exec.QueryRowContext(ctx, `SELECT first_scheduled_at FROM loop_generation_outputs
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
		runID, generation, nodeID, itemIndex).Scan(&raw); err != nil {
		return nil, fmt.Errorf("store: load parked Loop cell clock: %w", err)
	}
	if !raw.Valid {
		return nil, nil
	}
	parsed, err := parseLoopRunTimestamp(raw.String)
	if err != nil {
		return nil, fmt.Errorf("store: parse parked Loop cell clock: %w", err)
	}
	return parsed.UTC().Add(parkedFor).Format(time.RFC3339Nano), nil
}
