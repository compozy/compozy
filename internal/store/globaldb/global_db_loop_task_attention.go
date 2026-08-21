package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func projectLoopTaskRunAttentionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	diagnostic string,
	at time.Time,
) error {
	metadata, loopRun, err := loadBoundLoopTaskRunCell(ctx, exec, run)
	if err != nil {
		return err
	}
	var previousFlag string
	err = exec.QueryRowContext(
		ctx,
		`SELECT attention_flag FROM loop_node_controls WHERE loop_run_id = ? AND node_id = ?`,
		run.LoopRunID,
		metadata.NodeID,
	).Scan(&previousFlag)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: load Loop task attention: %w", err)
	}
	if at.IsZero() {
		at = run.QueuedAt.UTC()
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, attention_flag, attention_reason, revision, updated_at
	) VALUES (?, ?, ?, ?, 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		attention_flag = CASE
			WHEN attention_flag = '' OR attention_flag = ? THEN excluded.attention_flag
			ELSE attention_flag
		END,
		attention_reason = CASE
			WHEN attention_flag = '' OR attention_flag = ? THEN excluded.attention_reason
			ELSE attention_reason
		END,
		revision = revision + 1,
		updated_at = excluded.updated_at`,
		run.LoopRunID,
		metadata.NodeID,
		looppkg.AttentionWaitIntervention,
		diagnostic,
		at.UTC(),
		looppkg.AttentionSilence,
		looppkg.AttentionSilence,
	); err != nil {
		return fmt.Errorf("store: project Loop task attention: %w", err)
	}
	if previousFlag != "" && previousFlag != looppkg.AttentionSilence {
		return nil
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		loopRun.ID,
		loopRun.WorkspaceID,
		loopRunEventNodeAttentionFlagged,
		map[string]any{
			loopRunEventPayloadKeyGeneration:    metadata.Generation,
			loopRunEventPayloadKeyNodeID:        metadata.NodeID,
			loopRunEventPayloadKeyItemIndex:     metadata.ItemIndex,
			loopRunEventPayloadKeyTaskRunID:     run.ID,
			loopRunEventPayloadKeyAttentionFlag: looppkg.AttentionWaitIntervention,
			loopRunEventPayloadKeyReason:        diagnostic,
		},
		at.UTC(),
	)
}
