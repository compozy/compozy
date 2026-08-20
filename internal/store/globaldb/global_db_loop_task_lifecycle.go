package globaldb

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func writeCoordinatorGenerationSnapshotWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot taskpkg.GenerationSnapshot,
	finalizer taskpkg.GenerationStateFinalizer,
	actor taskpkg.ActorContext,
) error {
	if err := finalizer.WriteGenerationSnapshot(ctx, exec, snapshot); err != nil {
		return err
	}
	return parkQuarantinedLoopTasksWithExecutor(ctx, exec, snapshot, actor)
}

func parkQuarantinedLoopTasksWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot taskpkg.GenerationSnapshot,
	actor taskpkg.ActorContext,
) error {
	rows, err := exec.QueryContext(ctx, `SELECT outputs.node_id, outputs.item_index, controls.quarantined_at
		FROM loop_generation_outputs AS outputs
		JOIN loop_node_controls AS controls
		  ON controls.loop_run_id = outputs.loop_run_id AND controls.node_id = outputs.node_id
		WHERE outputs.loop_run_id = ? AND outputs.generation = ? AND outputs.status = ?
		  AND controls.quarantined = 1
		ORDER BY outputs.node_id, outputs.item_index`,
		snapshot.LoopRunID,
		snapshot.Generation,
		string(looppkg.AttemptQuarantined),
	)
	if err != nil {
		return fmt.Errorf("store: list quarantined Loop cell tasks: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("store: close quarantined Loop cell rows", "error", closeErr)
		}
	}()

	type quarantinedCell struct {
		nodeID      string
		itemIndex   int
		quarantined time.Time
	}
	cells := make([]quarantinedCell, 0, 4)
	for rows.Next() {
		var cell quarantinedCell
		if err := rows.Scan(&cell.nodeID, &cell.itemIndex, &cell.quarantined); err != nil {
			return fmt.Errorf("store: scan quarantined Loop cell task: %w", err)
		}
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate quarantined Loop cell tasks: %w", err)
	}
	for _, cell := range cells {
		taskID := looppkg.NodeCellTaskID(
			looppkg.RunID(snapshot.LoopRunID),
			snapshot.Generation,
			cell.nodeID,
			cell.itemIndex,
		)
		if err := parkQuarantinedLoopTaskWithExecutor(
			ctx,
			exec,
			taskID,
			cell.nodeID,
			snapshot.LoopRunID,
			actor,
			cell.quarantined,
		); err != nil {
			return err
		}
	}
	return nil
}

func parkQuarantinedLoopTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	nodeID string,
	loopRunID string,
	actor taskpkg.ActorContext,
	markedAt time.Time,
) error {
	record, err := (&TaskRepo{}).getTaskWithExecutor(ctx, exec, taskID)
	if err != nil {
		return err
	}
	if record.NeedsAttention != nil && record.Status == taskpkg.TaskStatusNeedsAttention {
		return nil
	}
	if record.NeedsAttention != nil || record.Status != taskpkg.TaskStatusReady {
		return fmt.Errorf(
			"%w: quarantined Loop task %q has status %q",
			taskpkg.ErrInvalidStatusTransition,
			taskID,
			record.Status,
		)
	}
	reason := fmt.Sprintf("loop node %s is quarantined; requeue it from the run to resume", nodeID)
	formattedAt := store.FormatTimestamp(markedAt)
	changed, err := exec.ExecContext(ctx, `UPDATE tasks SET
		needs_attention_reason = ?, needs_attention_at = ?,
		needs_attention_by_kind = ?, needs_attention_by_ref = ?, updated_at = ?
		WHERE id = ? AND status = ? AND needs_attention_at IS NULL`,
		reason,
		formattedAt,
		string(actor.Actor.Kind.Normalize()),
		actor.Actor.Ref,
		formattedAt,
		taskID,
		string(taskpkg.TaskStatusReady),
	)
	if err != nil {
		return fmt.Errorf("store: mark quarantined Loop task %q needs attention: %w", taskID, err)
	}
	affected, err := changed.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect quarantined Loop task %q attention write: %w", taskID, err)
	}
	if err := requireSingleTaskAttentionMutation(affected, taskID); err != nil {
		return err
	}
	return setTaskStatusWithEventContext(
		ctx,
		exec,
		taskID,
		taskpkg.TaskStatusReady,
		taskpkg.TaskStatusNeedsAttention,
		actor,
		markedAt,
		taskStatusEventContext{reason: reason, loopRunID: loopRunID},
	)
}

func requireSingleTaskAttentionMutation(affected int64, taskID string) error {
	if affected != 1 {
		return fmt.Errorf("%w: task %q attention changed", taskpkg.ErrConflict, taskID)
	}
	return nil
}
