package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func (g *LoopRepo) pauseNodeLane(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	mutation looppkg.NodePauseMutation,
	result *looppkg.NodePauseResult,
) error {
	itemIndex := *mutation.ItemIndex
	var status string
	if err := exec.QueryRowContext(ctx, `SELECT status FROM loop_generation_outputs
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
		mutation.RunID, run.Generation, mutation.NodeID, itemIndex).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: node lane %s/%d not found", looppkg.ErrValidation, mutation.NodeID, itemIndex)
		}
		return fmt.Errorf("store: load Loop node lane pause target: %w", err)
	}
	insert, err := exec.ExecContext(ctx, `INSERT OR IGNORE INTO loop_node_lane_pauses (
		workspace_id, loop_run_id, node_id, item_index, actor_kind, actor_id, reason, mode, requested_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, mutation.WorkspaceID, mutation.RunID, mutation.NodeID,
		itemIndex, mutation.Actor.Actor.Kind, mutation.Actor.Actor.Ref, mutation.Reason, mutation.Mode,
		mutation.RequestedAt.UTC())
	if err != nil {
		return fmt.Errorf("store: insert Loop node lane pause: %w", err)
	}
	affected, err := insert.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect Loop node lane pause: %w", err)
	}
	if affected == 0 {
		result.Control = pausedNodeControl(mutation, 1)
		return nil
	}
	if mutation.Mode == looppkg.NodePauseCancel {
		result.SessionIDs, err = listNodeCancellationSessions(ctx, exec, looppkg.CancellationMutation{
			WorkspaceID: mutation.WorkspaceID, RunID: mutation.RunID, NodeID: mutation.NodeID,
			ItemIndex: mutation.ItemIndex,
		})
		if err != nil {
			return err
		}
	}
	statuses := "'pending','enqueued','retrying'"
	if mutation.Mode == looppkg.NodePauseCancel {
		statuses += ",'running','control_pending','awaiting_goal'"
	}
	if _, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'paused', epoch = epoch + 1
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND status IN (`+statuses+`)`, mutation.RunID, run.Generation, mutation.NodeID, itemIndex); err != nil {
		return fmt.Errorf("store: park Loop node lane: %w", err)
	}
	if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID, loopRunEventNodePaused,
		nodePauseEventPayload(mutation, 1), mutation.RequestedAt); err != nil {
		return err
	}
	result.Control = pausedNodeControl(mutation, 1)
	result.Applied = true
	return nil
}

func (g *LoopRepo) resumeNodeLane(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	mutation looppkg.NodeResumeMutation,
	result *looppkg.NodeResumeResult,
) error {
	itemIndex := *mutation.ItemIndex
	var pausedAt time.Time
	if err := exec.QueryRowContext(ctx, `SELECT requested_at FROM loop_node_lane_pauses
		WHERE workspace_id = ? AND loop_run_id = ? AND node_id = ? AND item_index = ?`,
		mutation.WorkspaceID, mutation.RunID, mutation.NodeID, itemIndex).Scan(&pausedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return loopLifecycleReasonError(looppkg.ReasonCodeNodeNotPaused,
				fmt.Errorf("%w: node lane %s/%d is not paused", looppkg.ErrInvalidTransition,
					mutation.NodeID, itemIndex), nodeLifecycleStateActive,
				nodeLifecycleAllowedTransitions(nodeLifecycleStateActive), nil)
		}
		return fmt.Errorf("store: load Loop node lane pause: %w", err)
	}
	removed, err := exec.ExecContext(ctx, `DELETE FROM loop_node_lane_pauses
		WHERE workspace_id = ? AND loop_run_id = ? AND node_id = ? AND item_index = ?`,
		mutation.WorkspaceID, mutation.RunID, mutation.NodeID, itemIndex)
	if err != nil {
		return fmt.Errorf("store: release Loop node lane pause: %w", err)
	}
	affected, err := removed.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect Loop node lane resume: %w", err)
	}
	if affected != 1 {
		return loopLifecycleReasonError(looppkg.ReasonCodeNodeNotPaused,
			fmt.Errorf("%w: node lane %s/%d is not paused", looppkg.ErrInvalidTransition,
				mutation.NodeID, itemIndex), nodeLifecycleStateActive,
			nodeLifecycleAllowedTransitions(nodeLifecycleStateActive), nil)
	}
	pauseDuration := mutation.RequestedAt.UTC().Sub(pausedAt.UTC())
	if pauseDuration < 0 {
		pauseDuration = 0
	}
	shiftedAnchor, err := shiftedLoopCellFirstScheduledAt(
		ctx, exec, mutation.RunID, run.Generation, mutation.NodeID, itemIndex, pauseDuration,
	)
	if err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs
		SET first_scheduled_at = ?
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`, shiftedAnchor, mutation.RunID,
		run.Generation, mutation.NodeID, itemIndex); err != nil {
		return fmt.Errorf("store: shift Loop node lane action clock: %w", err)
	}
	statusExpression := "CASE WHEN next_attempt_at IS NOT NULL AND next_attempt_at > ? THEN 'retrying' ELSE 'pending' END"
	attemptExpression := watchEventsPayloadAttemptKey
	nextAttemptExpression := "next_attempt_at"
	args := []any{}
	if mutation.Mode == looppkg.NodeResumePlain {
		args = append(args, mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC())
	}
	if mutation.Mode == looppkg.NodeResumeImmediate {
		statusExpression = "'pending'"
		nextAttemptExpression = "NULL"
	}
	if mutation.Mode == looppkg.NodeResumeResetAttempts {
		statusExpression = "'pending'"
		attemptExpression = "1"
		nextAttemptExpression = "NULL"
	}
	args = append(args, mutation.RunID, run.Generation, mutation.NodeID, itemIndex)
	if _, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = `+statusExpression+`,
		attempt = `+attemptExpression+`, next_attempt_at = `+nextAttemptExpression+`, task_run_id = NULL,
		output_ref = CASE WHEN `+statusExpression+` = 'pending' THEN NULL ELSE output_ref END,
		epoch = epoch + 1 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND status = 'paused'`, args...); err != nil {
		return fmt.Errorf("store: restore Loop node lane: %w", err)
	}
	if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID, loopRunEventNodeResumed,
		nodeResumeEventPayload(mutation, 2), mutation.RequestedAt); err != nil {
		return err
	}
	coordinator, _, err := g.reserveLoopCoordinatorRunWithExecutor(ctx, exec, run, mutation.Actor.Origin,
		mutation.RequestedAt, "", fmt.Sprintf("loop.node.resume.%s.%s.%d.%d",
			mutation.RunID, mutation.NodeID, itemIndex, mutation.RequestedAt.UnixNano()))
	if err != nil {
		return err
	}
	result.Control = looppkg.NodeControl{LoopRunID: mutation.RunID, NodeID: mutation.NodeID,
		Paused: false, Revision: 2, UpdatedAt: mutation.RequestedAt.UTC()}
	result.Coordinator = &coordinator
	result.Applied = true
	return nil
}
