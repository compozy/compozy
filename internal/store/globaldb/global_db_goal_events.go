package globaldb

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
)

func appendGoalStatusChangedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	from string,
	to string,
	cause loop.ReasonCode,
	actorKind string,
	actorID string,
	at time.Time,
) error {
	sequence, changed, err := appendGoalStatusChangedRunEvent(
		ctx,
		exec,
		key,
		from,
		to,
		cause,
		actorKind,
		actorID,
		at,
	)
	if err != nil || !changed {
		return err
	}
	return enqueueGoalStatusOutboxIfSessionOrigin(ctx, exec, key, sequence, at)
}

func appendGoalStatusChangedRunEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	from string,
	to string,
	cause loop.ReasonCode,
	actorKind string,
	actorID string,
	at time.Time,
) (int64, bool, error) {
	if from == to {
		return 0, false, nil
	}
	sequence, err := appendLoopRunEventWithSequence(
		ctx,
		exec,
		key.LoopRunID,
		key.WorkspaceID,
		loopRunEventGoalStatusChanged,
		map[string]any{
			loopRunEventPayloadKeyNodeID:     string(key.NodeID),
			loopRunEventPayloadKeyItemIndex:  key.ItemIndex,
			loopRunEventPayloadKeyGeneration: key.Generation,
			loopRunEventPayloadKeyFrom:       from,
			loopRunEventPayloadKeyTo:         to,
			loopRunEventPayloadKeyCause:      string(cause),
			loopRunEventPayloadKeyActorKind:  actorKind,
			loopRunEventPayloadKeyActorID:    actorID,
		},
		at,
	)
	if err != nil {
		return 0, false, err
	}
	return sequence, true, nil
}
