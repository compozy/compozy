package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

var _ looppkg.NodeLivenessRecorder = (*LoopRepo)(nil)

// RecordNodeLiveness updates evidence or silence attention for one still-live task-run cell.
func (g *LoopRepo) RecordNodeLiveness(
	ctx context.Context,
	observation looppkg.NodeLivenessObservation,
) error {
	if err := g.checkReady(ctx, "record Loop node liveness"); err != nil {
		return err
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "record Loop node liveness", func(exec taskSQLExecutor) error {
		return recordNodeLivenessWithExecutor(ctx, exec, observation)
	})
}

type nodeLivenessCell struct {
	generation int
	nodeID     looppkg.NodeID
	itemIndex  int
	last       *time.Time
	attention  string
}

func recordNodeLivenessWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	observation looppkg.NodeLivenessObservation,
) error {
	cell, found, err := loadLiveLivenessCell(ctx, exec, observation)
	if err != nil || !found {
		return err
	}
	before := cell.attention
	if err := upsertNodeLivenessControl(ctx, exec, observation, cell); err != nil {
		return err
	}
	after, err := loadNodeAttention(ctx, exec, observation.LoopRunID, cell.nodeID)
	if err != nil {
		return err
	}
	if before == after {
		return nil
	}
	kind := loopRunEventNodeAttentionFlagged
	if after == "" {
		kind = loopRunEventNodeAttentionCleared
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		observation.LoopRunID,
		observation.WorkspaceID,
		kind,
		map[string]any{
			loopRunEventPayloadKeyGeneration: cell.generation,
			loopRunEventPayloadKeyNodeID:     cell.nodeID,
			loopRunEventPayloadKeyItemIndex:  cell.itemIndex,
			loopRunEventPayloadKeyReason:     looppkg.AttentionSilence,
		},
		observation.ObservedAt.UTC(),
	)
}

func loadLiveLivenessCell(
	ctx context.Context,
	exec taskSQLExecutor,
	observation looppkg.NodeLivenessObservation,
) (nodeLivenessCell, bool, error) {
	run, err := (&TaskRepo{}).getTaskRunWithExecutor(ctx, exec, observation.TaskRunID)
	if err != nil {
		return nodeLivenessCell{}, false, err
	}
	if run.LoopRunID != string(observation.LoopRunID) {
		return nodeLivenessCell{}, false, fmt.Errorf(
			"%w: liveness task run belongs to another Loop run",
			looppkg.ErrTransitionConflict,
		)
	}
	metadata, ok, err := loopNodeMetadataFromTaskRun(run.Metadata)
	if err != nil || !ok {
		return nodeLivenessCell{}, false, err
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, observation.LoopRunID)
	if err != nil {
		return nodeLivenessCell{}, false, err
	}
	if loopRun.WorkspaceID != observation.WorkspaceID || loopRun.Status.Terminal() || loopRun.CancelRequested {
		return nodeLivenessCell{}, false, nil
	}
	var status string
	err = exec.QueryRowContext(ctx, `SELECT status FROM loop_generation_outputs
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		  AND task_run_id = ? AND epoch = ?`,
		observation.LoopRunID, metadata.Generation, metadata.NodeID, metadata.ItemIndex,
		observation.TaskRunID, metadata.Epoch,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nodeLivenessCell{}, false, nil
	}
	if err != nil {
		return nodeLivenessCell{}, false, fmt.Errorf("store: load Loop liveness cell: %w", err)
	}
	switch status {
	case loopRunNodeOutputRunning, goalGenerationOutputEnqueued, loopGenerationOutputAwaitingGoal:
	default:
		return nodeLivenessCell{}, false, nil
	}
	cell := nodeLivenessCell{
		generation: metadata.Generation,
		nodeID:     looppkg.NodeID(metadata.NodeID),
		itemIndex:  metadata.ItemIndex,
	}
	var last sql.NullTime
	err = exec.QueryRowContext(ctx, `SELECT last_evidence_at, attention_flag FROM loop_node_controls
		WHERE loop_run_id = ? AND node_id = ?`, observation.LoopRunID, metadata.NodeID).Scan(&last, &cell.attention)
	if errors.Is(err, sql.ErrNoRows) {
		return cell, true, nil
	}
	if err != nil {
		return nodeLivenessCell{}, false, fmt.Errorf("store: load Loop node liveness control: %w", err)
	}
	if last.Valid {
		value := last.Time.UTC()
		cell.last = &value
	}
	return cell, true, nil
}

func upsertNodeLivenessControl(
	ctx context.Context,
	exec taskSQLExecutor,
	observation looppkg.NodeLivenessObservation,
	cell nodeLivenessCell,
) error {
	observedAt := observation.ObservedAt.UTC()
	last := cell.last
	if last == nil || observation.Evidence {
		last = &observedAt
	}
	attention := cell.attention
	reason := ""
	if observation.Evidence {
		if attention == looppkg.AttentionSilence {
			attention = ""
		}
	} else if observation.SilenceAfter > 0 && observedAt.Sub(last.UTC()) >= observation.SilenceAfter &&
		(attention == "" || attention == looppkg.AttentionSilence) {
		attention = looppkg.AttentionSilence
		reason = "No liveness evidence was observed within the configured silence window."
	}
	if attention == cell.attention && cell.last != nil && last.Equal(cell.last.UTC()) {
		return nil
	}
	deathStreak := "death_resume_streak"
	if observation.Evidence {
		deathStreak = "0"
	}
	_, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, attention_flag, attention_reason, last_evidence_at, death_resume_streak, revision, updated_at
	) VALUES (?, ?, ?, ?, ?, 0, 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		attention_flag = excluded.attention_flag,
		attention_reason = excluded.attention_reason,
		last_evidence_at = excluded.last_evidence_at,
		death_resume_streak = `+deathStreak+`,
		revision = loop_node_controls.revision + 1,
		updated_at = excluded.updated_at`,
		observation.LoopRunID, cell.nodeID, attention, reason, last.UTC(), observedAt,
	)
	if err != nil {
		return fmt.Errorf("store: upsert Loop node liveness control: %w", err)
	}
	return nil
}

func loadNodeAttention(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
) (string, error) {
	var attention string
	if err := exec.QueryRowContext(ctx, `SELECT attention_flag FROM loop_node_controls
		WHERE loop_run_id = ? AND node_id = ?`, runID, nodeID).Scan(&attention); err != nil {
		return "", fmt.Errorf("store: load Loop node attention: %w", err)
	}
	return attention, nil
}
