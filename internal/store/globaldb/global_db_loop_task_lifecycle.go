package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func writeCoordinatorGenerationSnapshotWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot taskpkg.GenerationSnapshot,
	continuationSnapshot *taskpkg.GenerationSnapshot,
	finalizer taskpkg.GenerationStateFinalizer,
	actor taskpkg.ActorContext,
) error {
	if err := finalizer.WriteGenerationSnapshot(ctx, exec, snapshot); err != nil {
		return err
	}
	return parkQuarantinedLoopTasksWithExecutor(
		ctx,
		exec,
		snapshot,
		continuationSnapshot,
		actor,
	)
}

func parkQuarantinedLoopTasksWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot taskpkg.GenerationSnapshot,
	continuationSnapshot *taskpkg.GenerationSnapshot,
	actor taskpkg.ActorContext,
) (err error) {
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
	defer func() { err = joinRowsCloseError(rows, err, "quarantined Loop cell rows") }()

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
		taskID, resolveErr := reservedQuarantinedLoopTaskID(
			snapshot,
			continuationSnapshot,
			cell.nodeID,
			cell.itemIndex,
		)
		if resolveErr != nil {
			return resolveErr
		}
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

func reservedQuarantinedLoopTaskID(
	snapshot taskpkg.GenerationSnapshot,
	continuationSnapshot *taskpkg.GenerationSnapshot,
	nodeID string,
	itemIndex int,
) (string, error) {
	runID := looppkg.RunID(snapshot.LoopRunID)
	currentTaskID := looppkg.NodeCellTaskID(runID, snapshot.Generation, nodeID, itemIndex)
	continuationGeneration, hasContinuation, err := quarantinedLoopContinuationGeneration(
		continuationSnapshot,
		nodeID,
		itemIndex,
	)
	if err != nil {
		return "", err
	}
	if hasContinuation {
		return looppkg.NodeCellTaskID(runID, continuationGeneration, nodeID, itemIndex), nil
	}
	// A pulse for an already-carried quarantined generation has no new
	// reservation. Reuse its deterministic cell task; the parking boundary
	// validates that the task exists and is either ready or already parked.
	return currentTaskID, nil
}

func quarantinedLoopContinuationOutputs(
	snapshot *taskpkg.GenerationSnapshot,
) ([]looppkg.GenerationOutput, error) {
	if snapshot == nil {
		return nil, nil
	}
	payload, err := looppkg.GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return nil, err
	}
	outputs := make([]looppkg.GenerationOutput, 0)
	for _, output := range payload.Outputs {
		if output.Status == string(looppkg.AttemptQuarantined) {
			outputs = append(outputs, output)
		}
	}
	return outputs, nil
}

func quarantinedLoopContinuationGeneration(
	snapshot *taskpkg.GenerationSnapshot,
	nodeID string,
	itemIndex int,
) (int, bool, error) {
	outputs, err := quarantinedLoopContinuationOutputs(snapshot)
	if err != nil {
		return 0, false, err
	}
	for _, output := range outputs {
		if output.NodeID == nodeID && output.ItemIndex == itemIndex {
			return snapshot.Generation, true, nil
		}
	}
	return 0, false, nil
}

func (g *TaskRepo) ensureQuarantinedLoopContinuationTasksWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	currentSnapshot taskpkg.GenerationSnapshot,
	continuationSnapshot *taskpkg.GenerationSnapshot,
	outputs []looppkg.GenerationOutput,
	parentTask taskpkg.Task,
	origin taskpkg.Origin,
	now time.Time,
) error {
	if continuationSnapshot == nil || len(outputs) == 0 {
		return nil
	}
	loopRunID := strings.TrimSpace(currentSnapshot.LoopRunID)
	if loopRunID == "" {
		loopRunID = strings.TrimSpace(continuationSnapshot.LoopRunID)
	}
	if loopRunID == "" || continuationSnapshot.Generation <= currentSnapshot.Generation {
		return fmt.Errorf("%w: quarantined Loop continuation identity is invalid", taskpkg.ErrValidation)
	}
	for _, output := range outputs {
		if err := g.ensureQuarantinedLoopContinuationTaskWithExecutor(
			ctx,
			exec,
			looppkg.RunID(loopRunID),
			currentSnapshot.Generation,
			continuationSnapshot.Generation,
			output,
			parentTask,
			origin,
			now,
		); err != nil {
			return err
		}
	}
	return nil
}

func (g *TaskRepo) ensureQuarantinedLoopContinuationTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	currentGeneration int,
	continuationGeneration int,
	output looppkg.GenerationOutput,
	parentTask taskpkg.Task,
	origin taskpkg.Origin,
	now time.Time,
) error {
	currentTaskID := looppkg.NodeCellTaskID(runID, currentGeneration, output.NodeID, output.ItemIndex)
	currentTask, err := g.getTaskWithExecutor(ctx, exec, currentTaskID)
	if err != nil {
		return fmt.Errorf("store: get quarantined Loop source task %q: %w", currentTaskID, err)
	}
	if err := g.validateQuarantinedLoopSourceTaskWithExecutor(ctx, exec, currentTask, output); err != nil {
		return err
	}
	metadata, err := quarantinedLoopContinuationMetadata(
		currentTask.Metadata,
		runID,
		continuationGeneration,
		output,
	)
	if err != nil {
		return err
	}
	continuationTaskID := looppkg.NodeCellTaskID(
		runID,
		continuationGeneration,
		output.NodeID,
		output.ItemIndex,
	)
	return g.createCoordinatorTaskIfMissingWithExecutor(
		ctx,
		exec,
		taskpkg.CoordinatorTaskSpec{
			TaskID: continuationTaskID,
			Title:  currentTask.Title,
			Description: fmt.Sprintf(
				"Generation %d node %s for loop run %s.",
				continuationGeneration,
				output.NodeID,
				runID,
			),
			Metadata: metadata,
		},
		parentTask,
		origin,
		now,
	)
}

func (g *TaskRepo) validateQuarantinedLoopSourceTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	currentTask taskpkg.Task,
	output looppkg.GenerationOutput,
) error {
	if currentTask.Status == taskpkg.TaskStatusCompleted {
		return nil
	}
	if currentTask.Status == taskpkg.TaskStatusReady && strings.TrimSpace(output.TaskRunID) != "" {
		currentRun, err := g.getTaskRunWithExecutor(ctx, exec, output.TaskRunID)
		if err != nil {
			return fmt.Errorf("store: get quarantined Loop source run %q: %w", output.TaskRunID, err)
		}
		if currentRun.TaskID == currentTask.ID && currentRun.Status.Normalize() == taskpkg.TaskRunStatusFailed {
			return nil
		}
	}
	return fmt.Errorf(
		"%w: quarantined Loop source task %q has status %q",
		taskpkg.ErrInvalidStatusTransition,
		currentTask.ID,
		currentTask.Status,
	)
}

func quarantinedLoopContinuationMetadata(
	raw json.RawMessage,
	runID looppkg.RunID,
	generation int,
	output looppkg.GenerationOutput,
) (json.RawMessage, error) {
	payload := make(map[string]any)
	if strings.TrimSpace(string(raw)) != "" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("store: decode quarantined Loop task metadata: %w", err)
		}
	}
	payload["loop_run_id"] = string(runID)
	payload["generation"] = generation
	payload["node_id"] = output.NodeID
	payload["item_index"] = output.ItemIndex
	payload["index"] = output.ItemIndex
	payload["attempt"] = output.Attempt
	payload["epoch"] = output.Epoch
	metadata, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("store: encode quarantined Loop continuation metadata: %w", err)
	}
	return metadata, nil
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
	if record.NeedsAttention == nil && record.Status == taskpkg.TaskStatusCompleted {
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
