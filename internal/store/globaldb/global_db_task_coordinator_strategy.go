package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func applyStrategyCancellationsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	intents []looppkg.StrategyCancellationIntent,
) error {
	for _, intent := range intents {
		requestResult, err := exec.ExecContext(ctx, `UPDATE loop_requests
			SET state = 'canceled', resolved_at = ?, actor_kind = ?, actor_id = ?
			WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
			AND state = 'pending'`, intent.At.UTC(), intent.ActorKind, intent.ActorID,
			run.ID, generation, intent.NodeID, intent.ItemIndex)
		if err != nil {
			return fmt.Errorf("store: cancel strategy request cell: %w", err)
		}
		affected, err := requestResult.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: inspect strategy request cancellation: %w", err)
		}
		if _, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET claim_state = 'claimed',
			claimed_by_kind = ?, claimed_by_id = ?, claimed_at = ? WHERE loop_run_id = ?
			AND generation = ? AND node_id = ? AND item_index = ?
			AND claim_state IN ('waiting','intervention_required')`, intent.ActorKind,
			intent.ActorID, intent.At.UTC(), run.ID, generation, intent.NodeID, intent.ItemIndex); err != nil {
			return fmt.Errorf("store: claim strategy-canceled wait cell: %w", err)
		}
		if affected == 0 {
			continue
		}
		if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventRequestCanceled, map[string]any{
				loopRunEventPayloadKeyGeneration: generation,
				loopRunEventPayloadKeyNodeID:     intent.NodeID,
				loopRunEventPayloadKeyItemIndex:  intent.ItemIndex,
				loopRunEventPayloadKeyActorKind:  intent.ActorKind,
				loopRunEventPayloadKeyActorID:    intent.ActorID,
				loopRunEventPayloadKeyReason:     intent.ReasonCode,
			}, intent.At.UTC()); err != nil {
			return err
		}
	}
	return nil
}
