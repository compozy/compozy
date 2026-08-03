package globaldb

import (
	"context"
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
	return insertLoopEffectIntentsWithExecutor(ctx, exec, run, eventID, event.Effects, at)
}

func appendNodeOutcomeEffectEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	kind := loopRunEventNodeSucceeded
	if event.Kind == looppkg.GenerationLifecycleEventNodeFailed {
		kind = loopRunEventNodeFailed
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
			"target":                         event.Target,
			networkWakeEventStateKey:         event.BreakerState,
		},
		at,
	)
}
