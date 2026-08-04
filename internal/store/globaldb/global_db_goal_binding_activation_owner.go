package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
)

func validateBindingActivationOwner(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.ActivateBindingRequest,
) error {
	if req.CellFence != nil {
		if req.CheckpointKey != nil || req.ExpectedControlEpoch != 0 || req.GrantID != 0 {
			return fmt.Errorf("%w: binding activation cannot mix cell and Goal owners", looppkg.ErrValidation)
		}
		return validateBindingActivationCell(ctx, exec, req.Key, *req.CellFence)
	}
	if req.CheckpointKey == nil {
		if req.ExpectedControlEpoch != 0 || req.GrantID != 0 {
			return goalControlStaleError("binding activation control identity is incomplete")
		}
		return nil
	}
	if req.ExpectedControlEpoch < 1 {
		return goalControlStaleError("binding activation control identity is incomplete")
	}
	return validateBindingActivationControlEpoch(ctx, exec, req)
}

func validateBindingActivationCell(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.BindingKey,
	fence goal.BindingCellFence,
) error {
	if err := fence.Key.Validate(); err != nil {
		return fmt.Errorf("store: validate binding activation cell fence: %w", err)
	}
	if fence.Key.WorkspaceID != key.WorkspaceID || fence.Key.LoopRunID != key.LoopRunID ||
		fence.Epoch < 0 || strings.TrimSpace(fence.TaskRunID) == "" {
		return fmt.Errorf("%w: binding activation cell owner is incomplete", looppkg.ErrValidation)
	}
	var exists int
	err := exec.QueryRowContext(
		ctx,
		`SELECT 1
		 FROM loop_generation_outputs AS output
		 JOIN loop_runs AS run ON run.id = output.loop_run_id
		 WHERE output.loop_run_id = ? AND run.workspace_id = ? AND run.status = 'running'
		   AND output.generation = ? AND output.node_id = ? AND output.item_index = ?
		   AND output.status IN ('enqueued', 'running') AND output.epoch = ? AND output.task_run_id = ?`,
		string(key.LoopRunID),
		string(key.WorkspaceID),
		fence.Key.Generation,
		string(fence.Key.NodeID),
		fence.Key.ItemIndex,
		fence.Epoch,
		strings.TrimSpace(fence.TaskRunID),
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return goalControlStaleError("binding activation cell owner is stale")
	}
	if err != nil {
		return fmt.Errorf("store: validate binding activation cell owner: %w", err)
	}
	return nil
}

func validateBindingActivationControlEpoch(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.ActivateBindingRequest,
) error {
	query := `SELECT 1 FROM loop_goal_checkpoints WHERE loop_run_id = ? AND control_epoch = ?`
	args := []any{string(req.Key.LoopRunID), req.ExpectedControlEpoch}
	if req.CheckpointKey != nil {
		if err := req.CheckpointKey.Validate(); err != nil {
			return fmt.Errorf("store: validate binding activation checkpoint: %w", err)
		}
		query += goalBindingCheckpointPredicate
		args = append(
			args,
			req.CheckpointKey.Generation,
			string(req.CheckpointKey.NodeID),
			req.CheckpointKey.ItemIndex,
		)
	}
	var exists int
	// dynamic-sql: checkpoint ownership is optional, so validation conditionally narrows the control epoch identity.
	err := exec.QueryRowContext(ctx, query, args...).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return goalControlStaleError("binding activation control epoch is stale")
	}
	if err != nil {
		return fmt.Errorf("store: validate binding activation control epoch: %w", err)
	}
	return nil
}
