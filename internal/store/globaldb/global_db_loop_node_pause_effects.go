package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func appendManualNodePauseEffects(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	mutation looppkg.NodePauseMutation,
	revision int64,
) error {
	resolved, err := loadWaitExecutedDefinition(ctx, exec, run)
	if err != nil {
		return err
	}
	node, found := loopWaitNodeByRuntimeID(resolved.Definition.Graph, "", string(mutation.NodeID))
	if !found {
		return fmt.Errorf(
			"%w: paused node %q is absent from the executed definition",
			looppkg.ErrValidation,
			mutation.NodeID,
		)
	}
	if len(node.OnPause) == 0 {
		return nil
	}
	var sourceID string
	err = exec.QueryRowContext(ctx, `SELECT id FROM loop_run_events WHERE loop_run_id = ?
		AND kind = ? AND json_extract(payload_json, '$.node_id') = ?
		AND json_extract(payload_json, '$.revision') = ? ORDER BY seq DESC LIMIT 1`,
		run.ID, loopRunEventNodePaused, mutation.NodeID, revision).Scan(&sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: node pause source event is missing", looppkg.ErrValidation)
	}
	if err != nil {
		return fmt.Errorf("store: load node pause source event: %w", err)
	}
	intents, err := looppkg.RenderEffectIntents(node.OnPause, looppkg.EffectContextRequest{
		Run: &run, Trigger: looppkg.EffectTriggerOnPause, Generation: max(1, run.Generation),
		NodeID: mutation.NodeID, ItemIndex: 0,
	})
	if err != nil {
		return err
	}
	return insertLoopEffectIntentsWithExecutor(ctx, exec, run, sourceID, intents, mutation.RequestedAt.UTC())
}
