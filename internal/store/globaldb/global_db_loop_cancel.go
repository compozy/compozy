package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

var _ looppkg.CancellationStore = (*LoopRepo)(nil)

const liveCancelOutputStatuses = "'pending','enqueued','running','retrying'," +
	"'waiting','paused','awaiting_child','control_pending','awaiting_goal'"

// RequestRunCancellation records first-writer Run intent and fences every live node cell.
func (g *LoopRepo) RequestRunCancellation(
	ctx context.Context,
	mutation looppkg.CancellationMutation,
) (looppkg.CancellationResult, error) {
	if err := g.checkReady(ctx, "request Loop run cancellation"); err != nil {
		return looppkg.CancellationResult{}, err
	}
	if err := mutation.Validate(false); err != nil {
		return looppkg.CancellationResult{}, err
	}
	var result looppkg.CancellationResult
	err := g.withTaskImmediateTransaction(ctx, "request Loop run cancellation", func(exec taskSQLExecutor) error {
		run, err := cancellationRun(ctx, exec, mutation)
		if err != nil {
			return err
		}
		result.Run = run
		if run.Status.Terminal() {
			result.Terminal = true
			return nil
		}
		applied, err := writeRunCancellationIntent(ctx, exec, mutation)
		if err != nil {
			return err
		}
		result.Applied = applied
		if !applied {
			result.Run, err = cancellationRun(ctx, exec, mutation)
			result.Terminal = result.Run.Status.Terminal()
			return err
		}
		if err := projectRunCancellation(ctx, exec, mutation); err != nil {
			return err
		}
		if err := claimCancellationWaits(ctx, exec, mutation, run); err != nil {
			return err
		}
		result.SessionIDs, err = listRunCancellationSessions(ctx, exec, mutation)
		if err != nil {
			return err
		}
		result.RevokedPromptLeases, err = cleanupRunCancellationGoals(ctx, exec, mutation)
		if err != nil {
			return err
		}
		live, err := runHasLiveCancellationOutputs(ctx, exec, mutation.RunID)
		if err != nil {
			return err
		}
		if mutation.Kind != looppkg.RunCancelKill && run.Status == looppkg.StatusRunning && live {
			return nil
		}
		err = terminalizeRunCancellation(ctx, exec, mutation, run)
		if err != nil {
			return err
		}
		result.Run.Status = looppkg.StatusCanceled
		result.Terminal = true
		return nil
	})
	if err != nil {
		return looppkg.CancellationResult{}, err
	}
	return result, nil
}

// AdvanceRunCancellation moves graceful delivery forward and terminalizes only at canceled.
func (g *LoopRepo) AdvanceRunCancellation(
	ctx context.Context,
	mutation looppkg.CancellationMutation,
	state looppkg.CancelState,
) (looppkg.CancellationResult, error) {
	if err := g.checkReady(ctx, "advance Loop run cancellation"); err != nil {
		return looppkg.CancellationResult{}, err
	}
	if err := mutation.Validate(false); err != nil {
		return looppkg.CancellationResult{}, err
	}
	if mutation.Kind != looppkg.RunCancelCancel || !validCancellationAdvance(state) {
		return looppkg.CancellationResult{}, fmt.Errorf("%w: invalid Run cancellation advance", looppkg.ErrValidation)
	}
	var result looppkg.CancellationResult
	err := g.withTaskImmediateTransaction(ctx, "advance Loop run cancellation", func(exec taskSQLExecutor) error {
		run, err := cancellationRun(ctx, exec, mutation)
		if err != nil {
			return err
		}
		result.Run = run
		if run.Status.Terminal() {
			result.Terminal = true
			return nil
		}
		if !run.CancelRequested || run.CancelKind != looppkg.RunCancelCancel {
			return fmt.Errorf("%w: Run cancellation intent changed", looppkg.ErrTransitionConflict)
		}
		result.Applied, err = advanceRunNodeCancelState(ctx, exec, mutation, state)
		if err != nil {
			return err
		}
		if state == looppkg.CancelStateDraining && result.Applied {
			result.Coordinator, err = g.reserveCancellationCoordinator(ctx, exec, run, mutation)
			return err
		}
		if state != looppkg.CancelStateCanceled {
			return nil
		}
		result.RevokedPromptLeases, err = cleanupRunCancellationGoals(ctx, exec, mutation)
		if err != nil {
			return err
		}
		if err := terminalizeRunCancellation(ctx, exec, mutation, run); err != nil {
			return err
		}
		result.Run.Status = looppkg.StatusCanceled
		result.Terminal = true
		return nil
	})
	if err != nil {
		return looppkg.CancellationResult{}, err
	}
	return result, nil
}

// RequestNodeCancellation records node intent and fences all live cells for that node.
func (g *LoopRepo) RequestNodeCancellation(
	ctx context.Context,
	mutation looppkg.CancellationMutation,
) (looppkg.CancellationResult, error) {
	if err := g.checkReady(ctx, "request Loop node cancellation"); err != nil {
		return looppkg.CancellationResult{}, err
	}
	if err := mutation.Validate(true); err != nil {
		return looppkg.CancellationResult{}, err
	}
	var result looppkg.CancellationResult
	err := g.withTaskImmediateTransaction(ctx, "request Loop node cancellation", func(exec taskSQLExecutor) error {
		run, err := cancellationRun(ctx, exec, mutation)
		if err != nil {
			return err
		}
		result.Run = run
		if run.Status.Terminal() || run.CancelRequested {
			result.Terminal = run.Status.Terminal()
			return nil
		}
		live, err := nodeHasLiveCancellationOutputs(ctx, exec, mutation.RunID, mutation.NodeID)
		if err != nil {
			return err
		}
		if !live {
			return nil
		}
		result.Applied, err = writeNodeCancellationIntent(ctx, exec, mutation)
		if err != nil {
			return err
		}
		if !result.Applied {
			return nil
		}
		if result.Applied {
			if err := fenceNodeCancellation(ctx, exec, mutation); err != nil {
				return err
			}
			if err := claimCancellationWaits(ctx, exec, mutation, run); err != nil {
				return err
			}
		}
		result.SessionIDs, err = listNodeCancellationSessions(ctx, exec, mutation)
		if err != nil || mutation.Kind != looppkg.RunCancelKill {
			return err
		}
		if err := finalizeNodeCancellation(ctx, exec, mutation, run); err != nil {
			return err
		}
		result.Coordinator, err = g.reserveCancellationCoordinator(ctx, exec, run, mutation)
		return err
	})
	if err != nil {
		return looppkg.CancellationResult{}, err
	}
	return result, nil
}

// AdvanceNodeCancellation advances cooperative delivery for one node.
func (g *LoopRepo) AdvanceNodeCancellation(
	ctx context.Context,
	mutation looppkg.CancellationMutation,
	state looppkg.CancelState,
) (looppkg.CancellationResult, error) {
	if err := g.checkReady(ctx, "advance Loop node cancellation"); err != nil {
		return looppkg.CancellationResult{}, err
	}
	if err := mutation.Validate(true); err != nil {
		return looppkg.CancellationResult{}, err
	}
	if mutation.Kind != looppkg.RunCancelCancel || !validCancellationAdvance(state) {
		return looppkg.CancellationResult{}, fmt.Errorf("%w: invalid node cancellation advance", looppkg.ErrValidation)
	}
	var result looppkg.CancellationResult
	err := g.withTaskImmediateTransaction(ctx, "advance Loop node cancellation", func(exec taskSQLExecutor) error {
		run, err := cancellationRun(ctx, exec, mutation)
		if err != nil {
			return err
		}
		result.Run = run
		if run.Status.Terminal() || run.CancelRequested {
			result.Terminal = run.Status.Terminal()
			return nil
		}
		result.Applied, err = advanceOneNodeCancelState(ctx, exec, mutation, state)
		if err != nil {
			return err
		}
		if state == looppkg.CancelStateDraining && result.Applied {
			result.Coordinator, err = g.reserveCancellationCoordinator(ctx, exec, run, mutation)
			return err
		}
		if state != looppkg.CancelStateCanceled {
			return nil
		}
		if err := finalizeNodeCancellation(ctx, exec, mutation, run); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return looppkg.CancellationResult{}, err
	}
	return result, nil
}

func cancellationRun(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) (looppkg.Run, error) {
	run, err := getLoopRunByIDWithExecutor(ctx, exec, mutation.RunID)
	if err != nil {
		return looppkg.Run{}, err
	}
	if run.WorkspaceID != mutation.WorkspaceID {
		return looppkg.Run{}, fmt.Errorf("%w: %s", looppkg.ErrRunNotFound, mutation.RunID)
	}
	return run, nil
}

func validCancellationAdvance(state looppkg.CancelState) bool {
	switch state {
	case looppkg.CancelStateDelivering, looppkg.CancelStateDraining, looppkg.CancelStateCanceled:
		return true
	default:
		return false
	}
}

func writeRunCancellationIntent(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) (bool, error) {
	result, err := exec.ExecContext(ctx, `UPDATE loop_runs SET
		cancel_requested = 1, cancel_kind = ?, control_actor_kind = ?, control_actor_id = ?, control_requested_at = ?
		WHERE id = ? AND cancel_requested = 0`, mutation.Kind.String(),
		mutation.Actor.Actor.Kind.Normalize(), strings.TrimSpace(mutation.Actor.Actor.Ref),
		mutation.RequestedAt.UTC(), mutation.RunID)
	if err != nil {
		return false, fmt.Errorf("store: request Loop run cancellation: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func projectRunCancellation(ctx context.Context, exec taskSQLExecutor, mutation looppkg.CancellationMutation) error {
	state := looppkg.CancelStateRequested
	if mutation.Kind == looppkg.RunCancelKill {
		state = looppkg.CancelStateCanceled
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, cancel_state, cancel_actor_kind, cancel_actor_id, cancel_reason,
		cancel_requested_at, revision, updated_at
	) SELECT DISTINCT loop_run_id, node_id, ?, ?, ?, ?, ?, 1, ?
	FROM loop_generation_outputs WHERE loop_run_id = ? AND status IN (`+liveCancelOutputStatuses+`)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		cancel_state = CASE WHEN loop_node_controls.cancel_state = ''
			THEN excluded.cancel_state ELSE loop_node_controls.cancel_state END,
		cancel_actor_kind = CASE WHEN loop_node_controls.cancel_state = ''
			THEN excluded.cancel_actor_kind ELSE loop_node_controls.cancel_actor_kind END,
		cancel_actor_id = CASE WHEN loop_node_controls.cancel_state = ''
			THEN excluded.cancel_actor_id ELSE loop_node_controls.cancel_actor_id END,
		cancel_reason = CASE WHEN loop_node_controls.cancel_state = ''
			THEN excluded.cancel_reason ELSE loop_node_controls.cancel_reason END,
		cancel_requested_at = CASE WHEN loop_node_controls.cancel_state = ''
			THEN excluded.cancel_requested_at ELSE loop_node_controls.cancel_requested_at END,
		revision = CASE WHEN loop_node_controls.cancel_state = ''
			THEN loop_node_controls.revision + 1 ELSE loop_node_controls.revision END,
		updated_at = CASE WHEN loop_node_controls.cancel_state = ''
			THEN excluded.updated_at ELSE loop_node_controls.updated_at END`,
		state, mutation.Actor.Actor.Kind.Normalize(), strings.TrimSpace(mutation.Actor.Actor.Ref),
		strings.TrimSpace(mutation.Reason), mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC(),
		mutation.RunID,
	); err != nil {
		return fmt.Errorf("store: project Loop run cancellation: %w", err)
	}
	_, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET epoch = epoch + 1
		WHERE loop_run_id = ? AND status IN (`+liveCancelOutputStatuses+`)`, mutation.RunID)
	if err != nil {
		return fmt.Errorf("store: fence Loop run cancellation: %w", err)
	}
	return nil
}

func writeNodeCancellationIntent(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) (bool, error) {
	state := looppkg.CancelStateRequested
	if mutation.Kind == looppkg.RunCancelKill {
		state = looppkg.CancelStateCanceled
	}
	result, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, cancel_state, cancel_actor_kind, cancel_actor_id, cancel_reason,
		cancel_requested_at, revision, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		cancel_state = excluded.cancel_state, cancel_actor_kind = excluded.cancel_actor_kind,
		cancel_actor_id = excluded.cancel_actor_id, cancel_reason = excluded.cancel_reason,
		cancel_requested_at = excluded.cancel_requested_at, revision = loop_node_controls.revision + 1,
		updated_at = excluded.updated_at
	WHERE loop_node_controls.cancel_state = ''`, mutation.RunID, mutation.NodeID, state,
		mutation.Actor.Actor.Kind.Normalize(), strings.TrimSpace(mutation.Actor.Actor.Ref),
		strings.TrimSpace(mutation.Reason),
		mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC())
	if err != nil {
		return false, fmt.Errorf("store: request Loop node cancellation: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func fenceNodeCancellation(ctx context.Context, exec taskSQLExecutor, mutation looppkg.CancellationMutation) error {
	_, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET epoch = epoch + 1
		WHERE loop_run_id = ? AND node_id = ? AND status IN (`+liveCancelOutputStatuses+`)`,
		mutation.RunID, mutation.NodeID)
	if err != nil {
		return fmt.Errorf("store: fence Loop node cancellation: %w", err)
	}
	return nil
}

func advanceRunNodeCancelState(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	state looppkg.CancelState,
) (bool, error) {
	from := looppkg.CancelStateRequested
	switch state {
	case looppkg.CancelStateDraining:
		from = looppkg.CancelStateDelivering
	case looppkg.CancelStateCanceled:
		from = looppkg.CancelStateDraining
	}
	result, err := exec.ExecContext(
		ctx,
		`UPDATE loop_node_controls SET cancel_state = ?, revision = revision + 1, updated_at = ?
		WHERE loop_run_id = ? AND cancel_state = ?`,
		state,
		mutation.RequestedAt.UTC(),
		mutation.RunID,
		from,
	)
	if err != nil {
		return false, fmt.Errorf("store: advance Loop run node cancellation: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func advanceOneNodeCancelState(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	state looppkg.CancelState,
) (bool, error) {
	from := looppkg.CancelStateRequested
	switch state {
	case looppkg.CancelStateDraining:
		from = looppkg.CancelStateDelivering
	case looppkg.CancelStateCanceled:
		from = looppkg.CancelStateDraining
	}
	result, err := exec.ExecContext(
		ctx,
		`UPDATE loop_node_controls SET cancel_state = ?, revision = revision + 1, updated_at = ?
		WHERE loop_run_id = ? AND node_id = ? AND cancel_state = ?`,
		state,
		mutation.RequestedAt.UTC(),
		mutation.RunID,
		mutation.NodeID,
		from,
	)
	if err != nil {
		return false, fmt.Errorf("store: advance Loop node cancellation: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func cleanupRunCancellationGoals(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) ([]looppkg.GoalPromptLease, error) {
	request := goalCancellationRequest{
		WorkspaceID: mutation.WorkspaceID, RunID: mutation.RunID,
		Actor: mutation.Actor, CanceledAt: mutation.RequestedAt.UTC(),
	}
	leases, err := cancelGoalCheckpointsWithExecutor(ctx, exec, request, true)
	if err != nil {
		return nil, err
	}
	if err := closeCanceledGoalBindings(ctx, exec, request); err != nil {
		return nil, err
	}
	return leases, nil
}

func terminalizeRunCancellation(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	run looppkg.Run,
) error {
	if err := claimCancellationWaits(ctx, exec, mutation, run); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'canceled',
		next_attempt_at = NULL WHERE loop_run_id = ? AND status IN (`+liveCancelOutputStatuses+`)`, mutation.RunID); err != nil {
		return fmt.Errorf("store: terminalize canceled Loop outputs: %w", err)
	}
	cause := looppkg.TransitionCauseOperatorCancel
	if mutation.Kind == looppkg.RunCancelKill {
		cause = looppkg.TransitionCauseOperatorKill
	}
	if err := compareAndSwapLoopRunStatusWithEffects(
		ctx,
		exec,
		mutation.RunID,
		run.Status,
		looppkg.StatusCanceled,
		cause,
		mutation.Effects,
		mutation.RequestedAt.UTC(),
	); err != nil {
		return err
	}
	return nil
}
