package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func (g *LoopRepo) requestNodeLaneCancellation(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	run looppkg.Run,
	result *looppkg.CancellationResult,
) error {
	itemIndex := *mutation.ItemIndex
	live, err := nodeLaneLive(ctx, exec, mutation.RunID, run.Generation, mutation.NodeID, itemIndex)
	if err != nil {
		return err
	}
	if !live {
		return nil
	}
	sessions, err := listNodeCancellationSessions(ctx, exec, mutation)
	if err != nil {
		return err
	}
	if err := prepareNodeLaneCancellation(ctx, exec, mutation, run); err != nil {
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
		MAX(output.attempt, 1), 'cancellation', ?, ?, 'canceled',
		COALESCE(task_run.started_at, task_run.claimed_at, task_run.queued_at, ?), ?
		FROM loop_generation_outputs AS output LEFT JOIN task_runs AS task_run ON task_run.id = output.task_run_id
		WHERE output.loop_run_id = ? AND output.generation = ? AND output.node_id = ?
		AND output.item_index = ? AND output.status IN (`+liveCancelOutputStatuses+`)
		ON CONFLICT(loop_run_id, generation, node_id, item_index, attempt) DO NOTHING`,
		failureCode, cause, mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC(), mutation.RunID,
		run.Generation, mutation.NodeID, itemIndex); err != nil {
		return fmt.Errorf("store: append canceled Loop node lane attempt: %w", err)
	}
	if _, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'canceled',
		next_attempt_at = NULL, task_run_id = NULL, epoch = epoch + 1 WHERE loop_run_id = ?
		AND generation = ? AND node_id = ? AND item_index = ? AND status IN (`+liveCancelOutputStatuses+`)`,
		mutation.RunID, run.Generation, mutation.NodeID, itemIndex); err != nil {
		return fmt.Errorf("store: cancel Loop node lane: %w", err)
	}
	kind := loopRunEventNodeCanceled
	if mutation.Kind == looppkg.RunCancelKill {
		kind = loopRunEventNodeKilled
	}
	eventID, _, err := appendLoopRunEventWithIdentity(ctx, exec, run.ID, run.WorkspaceID, kind, map[string]any{
		loopRunEventPayloadKeyGeneration: run.Generation,
		loopRunEventPayloadKeyNodeID:     mutation.NodeID,
		loopRunEventPayloadKeyItemIndex:  itemIndex,
		loopRunEventPayloadKeyActorKind:  mutation.Actor.Actor.Kind.Normalize(),
		loopRunEventPayloadKeyActorID:    strings.TrimSpace(mutation.Actor.Actor.Ref),
		loopRunEventPayloadKeyReason:     strings.TrimSpace(mutation.Reason),
	}, mutation.RequestedAt.UTC())
	if err != nil {
		return err
	}
	if mutation.Kind == looppkg.RunCancelCancel && len(mutation.Effects) > 0 {
		if err := insertLoopEffectIntentsWithExecutor(
			ctx, exec, run, eventID, mutation.Effects, mutation.RequestedAt.UTC(),
		); err != nil {
			return err
		}
	}
	coordinator, err := g.reserveCancellationCoordinator(ctx, exec, run, mutation)
	if err != nil {
		return err
	}
	result.SessionIDs = sessions
	result.Coordinator = coordinator
	result.Applied = true
	return nil
}

func prepareNodeLaneCancellation(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	run looppkg.Run,
) error {
	if err := claimCancellationWaits(ctx, exec, mutation, run); err != nil {
		return err
	}
	return closeCanceledRunAgentLaneBinding(ctx, exec, mutation, run.Generation)
}

func nodeLaneLive(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	generation int,
	nodeID looppkg.NodeID,
	itemIndex int,
) (bool, error) {
	var live bool
	err := exec.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM loop_generation_outputs
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND status IN (`+liveCancelOutputStatuses+`))`, runID, generation, nodeID, itemIndex).Scan(&live)
	if err != nil {
		return false, fmt.Errorf("store: inspect live Loop node lane: %w", err)
	}
	return live, nil
}
