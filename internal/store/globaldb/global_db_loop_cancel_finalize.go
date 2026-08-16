package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func finalizeNodeCancellation(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	run looppkg.Run,
) error {
	if err := claimCancellationWaits(ctx, exec, mutation, run); err != nil {
		return err
	}
	failureCode := string(looppkg.TransitionCauseOperatorCancel)
	if mutation.Kind == looppkg.RunCancelKill {
		failureCode = string(looppkg.TransitionCauseOperatorKill)
	}
	cause := strings.TrimSpace(mutation.Reason)
	if cause == "" {
		cause = failureCode
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_attempts (
		loop_run_id, generation, node_id, item_index, attempt, failure_class, failure_code,
		cause, disposition, started_at, ended_at
	) SELECT output.loop_run_id, output.generation, output.node_id, output.item_index,
		CASE WHEN output.status = 'retrying' THEN output.attempt + 1 ELSE MAX(output.attempt, 1) END,
		'cancellation', ?, ?, 'canceled',
		COALESCE(task_run.started_at, task_run.claimed_at, task_run.queued_at, ?), ?
	FROM loop_generation_outputs AS output
	LEFT JOIN task_runs AS task_run ON task_run.id = output.task_run_id
	WHERE output.loop_run_id = ? AND output.node_id = ?
	  AND output.status IN (`+liveCancelOutputStatuses+`)
	ON CONFLICT(loop_run_id, generation, node_id, item_index, attempt) DO NOTHING`,
		failureCode, cause, mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC(),
		mutation.RunID, mutation.NodeID); err != nil {
		return fmt.Errorf("store: append canceled Loop node attempts: %w", err)
	}
	if _, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'canceled', next_attempt_at = NULL
		WHERE loop_run_id = ? AND node_id = ? AND status IN (`+liveCancelOutputStatuses+`)`,
		mutation.RunID, mutation.NodeID); err != nil {
		return fmt.Errorf("store: terminalize canceled Loop node outputs: %w", err)
	}
	kind := loopRunEventNodeCanceled
	if mutation.Kind == looppkg.RunCancelKill {
		kind = loopRunEventNodeKilled
	}
	eventID, _, err := appendLoopRunEventWithIdentity(ctx, exec, run.ID, run.WorkspaceID, kind, map[string]any{
		loopRunEventPayloadKeyGeneration: run.Generation,
		loopRunEventPayloadKeyNodeID:     mutation.NodeID,
		loopRunEventPayloadKeyActorKind:  mutation.Actor.Actor.Kind.Normalize(),
		loopRunEventPayloadKeyActorID:    strings.TrimSpace(mutation.Actor.Actor.Ref),
		loopRunEventPayloadKeyReason:     strings.TrimSpace(mutation.Reason),
	}, mutation.RequestedAt.UTC())
	if err != nil {
		return err
	}
	if mutation.Kind == looppkg.RunCancelKill || len(mutation.Effects) == 0 {
		return nil
	}
	return insertLoopEffectIntentsWithExecutor(
		ctx, exec, run, eventID, mutation.Effects, mutation.RequestedAt.UTC(),
	)
}

func claimCancellationWaits(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	run looppkg.Run,
) error {
	requestQuery := `SELECT generation, node_id, item_index FROM loop_requests
		WHERE loop_run_id = ? AND state = 'pending'`
	requestArgs := []any{mutation.RunID}
	if mutation.NodeID != "" {
		requestQuery += ` AND node_id = ?`
		requestArgs = append(requestArgs, mutation.NodeID)
	}
	if mutation.ItemIndex != nil {
		requestQuery += ` AND item_index = ?`
		requestArgs = append(requestArgs, *mutation.ItemIndex)
	}
	rows, err := exec.QueryContext(ctx, requestQuery, requestArgs...)
	if err != nil {
		return fmt.Errorf("store: list requests for cancellation: %w", err)
	}
	type requestRef struct {
		generation int
		nodeID     string
		itemIndex  int
	}
	requests := make([]requestRef, 0)
	for rows.Next() {
		var request requestRef
		if err := rows.Scan(&request.generation, &request.nodeID, &request.itemIndex); err != nil {
			return errors.Join(
				fmt.Errorf("store: scan request for cancellation: %w", err),
				closeRowsWithContext(rows, "requests for cancellation"),
			)
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return errors.Join(
			fmt.Errorf("store: iterate requests for cancellation: %w", err),
			closeRowsWithContext(rows, "requests for cancellation"),
		)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("store: close requests for cancellation: %w", err)
	}
	actorKind := mutation.Actor.Actor.Kind.Normalize()
	actorID := strings.TrimSpace(mutation.Actor.Actor.Ref)
	requestUpdate := `UPDATE loop_requests SET state = 'canceled', resolved_at = ?,
		actor_kind = ?, actor_id = ? WHERE loop_run_id = ? AND state = 'pending'`
	updateArgs := []any{mutation.RequestedAt.UTC(), actorKind, actorID, mutation.RunID}
	if mutation.NodeID != "" {
		requestUpdate += ` AND node_id = ?`
		updateArgs = append(updateArgs, mutation.NodeID)
	}
	if mutation.ItemIndex != nil {
		requestUpdate += ` AND item_index = ?`
		updateArgs = append(updateArgs, *mutation.ItemIndex)
	}
	if _, err := exec.ExecContext(ctx, requestUpdate, updateArgs...); err != nil {
		return fmt.Errorf("store: cancel Loop requests: %w", err)
	}
	for _, request := range requests {
		if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventRequestCanceled, map[string]any{
				loopRunEventPayloadKeyGeneration: request.generation,
				loopRunEventPayloadKeyNodeID:     request.nodeID,
				loopRunEventPayloadKeyItemIndex:  request.itemIndex,
				loopRunEventPayloadKeyActorKind:  actorKind,
				loopRunEventPayloadKeyActorID:    actorID,
			}, mutation.RequestedAt.UTC()); err != nil {
			return err
		}
	}
	query := `UPDATE loop_node_waits SET claim_state = 'claimed', claimed_by_kind = ?,
		claimed_by_id = ?, claimed_at = ?
		WHERE loop_run_id = ? AND claim_state IN ('waiting','intervention_required')`
	args := []any{
		mutation.Actor.Actor.Kind.Normalize(),
		strings.TrimSpace(mutation.Actor.Actor.Ref),
		mutation.RequestedAt.UTC(),
		mutation.RunID,
	}
	if mutation.NodeID != "" {
		query += " AND node_id = ?"
		args = append(args, mutation.NodeID)
	}
	if mutation.ItemIndex != nil {
		query += " AND item_index = ?"
		args = append(args, *mutation.ItemIndex)
	}
	if _, err := exec.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("store: claim canceled Loop node waits: %w", err)
	}
	return nil
}

func closeRowsWithContext(rows *sql.Rows, operation string) error {
	if err := rows.Close(); err != nil {
		return fmt.Errorf("store: close %s: %w", operation, err)
	}
	return nil
}
