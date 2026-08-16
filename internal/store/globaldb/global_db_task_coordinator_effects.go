package globaldb

import (
	"context"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func appendRetryScheduledEffectEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	payload := map[string]any{
		loopRunEventPayloadKeyGeneration:  generation,
		loopRunEventPayloadKeyNodeID:      event.NodeID,
		loopRunEventPayloadKeyItemIndex:   event.ItemIndex,
		watchEventsPayloadAttemptKey:      event.Attempt,
		loopRunEventPayloadKeyIssuedEpoch: event.IssuedEpoch,
		"next_attempt_at":                 event.NextAttemptAt,
		"failure_class":                   event.FailureClass,
	}
	eventID, _, err := appendLoopRunEventWithIdentity(
		ctx,
		exec,
		run.ID,
		run.WorkspaceID,
		loopRunEventNodeRetryScheduled,
		payload,
		at,
	)
	if err != nil {
		return err
	}
	if event.Kind == looppkg.GenerationLifecycleEventNodeCanceled {
		if err := appendCanceledRequestEventsForNode(ctx, exec, run, generation, event.NodeID, at); err != nil {
			return err
		}
	}
	return insertLoopEffectIntentsWithExecutor(ctx, exec, run, eventID, event.Effects, at)
}

func appendCanceledRequestEventsForNode(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	nodeID string,
	at time.Time,
) (err error) {
	rows, err := exec.QueryContext(ctx, `SELECT item_index, actor_kind, actor_id FROM loop_requests
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND state = 'canceled'
		AND resolved_at = ?`, run.ID, generation, nodeID, at.UTC())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close canceled Loop request rows: %w", closeErr))
		}
	}()
	type canceledRequest struct {
		itemIndex int
		actorKind string
		actorID   string
	}
	requests := make([]canceledRequest, 0)
	for rows.Next() {
		var request canceledRequest
		if err := rows.Scan(&request.itemIndex, &request.actorKind, &request.actorID); err != nil {
			return err
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, request := range requests {
		if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventRequestCanceled, map[string]any{
				loopRunEventPayloadKeyGeneration: generation,
				loopRunEventPayloadKeyNodeID:     nodeID,
				loopRunEventPayloadKeyItemIndex:  request.itemIndex,
				loopRunEventPayloadKeyActorKind:  request.actorKind,
				loopRunEventPayloadKeyActorID:    request.actorID,
			}, at); err != nil {
			return err
		}
	}
	return nil
}

func appendNodeOutcomeEffectEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	var kind string
	switch event.Kind {
	case looppkg.GenerationLifecycleEventNodeCanceled:
		kind = loopRunEventNodeCanceled
	case looppkg.GenerationLifecycleEventNodeFailed:
		kind = loopRunEventNodeFailed
	default:
		kind = loopRunEventNodeSucceeded
	}
	payload := map[string]any{
		loopRunEventPayloadKeyGeneration: generation,
		loopRunEventPayloadKeyNodeID:     event.NodeID,
		loopRunEventPayloadKeyItemIndex:  event.ItemIndex,
		watchEventsPayloadAttemptKey:     event.Attempt,
		"disposition":                    event.Disposition,
	}
	if event.Failure != nil {
		payload[loopRunEventPayloadKeyFailure] = event.Failure
	}
	eventID, _, err := appendLoopRunEventWithIdentity(
		ctx,
		exec,
		run.ID,
		run.WorkspaceID,
		kind,
		payload,
		at,
	)
	if err != nil {
		return err
	}
	return insertLoopEffectIntentsWithExecutor(ctx, exec, run, eventID, event.Effects, at)
}

func appendNodeQuarantinedEffectEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	payload := map[string]any{
		loopRunEventPayloadKeyGeneration: generation,
		loopRunEventPayloadKeyNodeID:     event.NodeID,
		loopRunEventPayloadKeyItemIndex:  event.ItemIndex,
		watchEventsPayloadAttemptKey:     event.Attempt,
		"disposition":                    event.Disposition,
		loopRunEventPayloadKeyFailure:    event.Failure,
		"entry":                          event.QuarantineEntry,
	}
	eventID, _, err := appendLoopRunEventWithIdentity(
		ctx, exec, run.ID, run.WorkspaceID, loopRunEventNodeQuarantined, payload, at,
	)
	if err != nil {
		return err
	}
	return insertLoopEffectIntentsWithExecutor(ctx, exec, run, eventID, event.Effects, at)
}

func appendTargetBreakerTransitionEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		run.ID,
		run.WorkspaceID,
		loopRunEventTargetBreakerTransition,
		map[string]any{
			loopRunEventPayloadKeyGeneration: generation,
			"family":                         event.TargetFamily,
			loopRunEventPayloadKeyTarget:     event.Target,
			networkWakeEventStateKey:         event.BreakerState,
		},
		at,
	)
}
