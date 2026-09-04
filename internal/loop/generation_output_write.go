package loop

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/task"
)

func writeGenerationOutput(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	generation int,
	output GenerationOutput, resolvedRuntime any,
) error {
	expectedEpoch := output.Epoch
	if output.ExpectedEpoch != nil {
		expectedEpoch = *output.ExpectedEpoch
	}
	// Fence stale same-epoch snapshots while preserving completed run-loop await refinement.
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_id, artifact_name,
			output_ref, task_run_id, child_loop_run_id, resolved_runtime_json, attempt, next_attempt_at,
			first_scheduled_at, epoch
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(loop_run_id, generation, node_id, item_index) DO UPDATE SET
			status = excluded.status,
			output_id = excluded.output_id,
			artifact_name = excluded.artifact_name,
			output_ref = excluded.output_ref,
			task_run_id = excluded.task_run_id,
			child_loop_run_id = excluded.child_loop_run_id,
			resolved_runtime_json = COALESCE(
				excluded.resolved_runtime_json,
				loop_generation_outputs.resolved_runtime_json
			),
			attempt = excluded.attempt,
			next_attempt_at = excluded.next_attempt_at,
			first_scheduled_at = COALESCE(
				loop_generation_outputs.first_scheduled_at,
				excluded.first_scheduled_at
			),
			epoch = excluded.epoch
		WHERE loop_generation_outputs.epoch = ? AND NOT (
			loop_generation_outputs.epoch = excluded.epoch
			AND loop_generation_outputs.status IN ('succeeded', 'partial', 'failed', 'canceled', 'quarantined')
			AND excluded.status IN ('pending', 'enqueued', 'running', 'retrying', 'waiting',
				'paused', 'awaiting_child', 'control_pending', 'awaiting_goal')
			AND NOT (
				loop_generation_outputs.status = 'succeeded'
				AND excluded.status = 'awaiting_child'
				AND excluded.task_run_id IS NOT NULL
				AND loop_generation_outputs.task_run_id IS excluded.task_run_id
				AND loop_generation_outputs.child_loop_run_id IS NULL AND excluded.child_loop_run_id IS NOT NULL
			)
		  )`,
		loopRunID,
		generation,
		output.NodeID,
		output.ItemIndex,
		output.Status,
		sqlNullString(output.OutputID),
		sqlNullString(output.ArtifactName),
		sqlNullString(output.OutputRef),
		sqlNullString(output.TaskRunID),
		sqlNullString(output.ChildLoopRunID),
		resolvedRuntime,
		output.Attempt,
		output.NextAttemptAt,
		output.FirstScheduledAt,
		output.Epoch,
		expectedEpoch,
	)
	if err != nil {
		return fmt.Errorf("loop: write generation output %s/%d: %w", output.NodeID, output.ItemIndex, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("loop: inspect generation output %s/%d write: %w", output.NodeID, output.ItemIndex, err)
	}
	if affected == 0 {
		return staleGenerationOutputError{
			nodeID:        output.NodeID,
			itemIndex:     output.ItemIndex,
			expectedEpoch: expectedEpoch,
		}
	}
	return nil
}
