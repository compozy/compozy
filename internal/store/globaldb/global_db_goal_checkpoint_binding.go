package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// BindCheckpoint adopts the exact active binding under the checkpoint owner fence.
func (g *GoalRepo) BindCheckpoint(ctx context.Context, req goal.BindCheckpointRequest) (goal.Checkpoint, error) {
	if err := g.checkReady(ctx, "bind Goal checkpoint"); err != nil {
		return goal.Checkpoint{}, err
	}
	if err := validateBindCheckpointRequest(req); err != nil {
		return goal.Checkpoint{}, err
	}
	var updated goal.Checkpoint
	err := g.withTaskImmediateTransaction(ctx, "bind Goal checkpoint", func(exec taskSQLExecutor) error {
		if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
			return err
		}
		checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
		if err != nil {
			return err
		}
		if checkpoint.ControlEpoch != req.ExpectedControlEpoch || checkpoint.BindingEpoch != req.ExpectedBindingEpoch ||
			checkpoint.Phase != req.ExpectedPhase || checkpoint.Status != goalStatusActive ||
			checkpoint.TaskRunID != req.TaskRunID {
			return goalControlStaleError("Goal checkpoint binding owner changed")
		}
		if err := validateActiveGoalPromptBinding(
			ctx,
			exec,
			req.Key,
			req.BindingHandle,
			req.BindingEpoch,
			req.SessionID,
		); err != nil {
			return err
		}
		if checkpoint.BindingEpoch == req.BindingEpoch && checkpoint.SessionID == req.SessionID &&
			checkpoint.BindingHandle == req.BindingHandle {
			updated = checkpoint
			return nil
		}
		if checkpoint.Phase != goalCheckpointPhaseIdle || checkpoint.PromptID != "" || checkpoint.QueueEntryID != "" {
			return goalControlStaleError("Goal checkpoint cannot change binding while a prompt is pending")
		}
		affected, err := sqlcgen.New(exec).BindGoalCheckpoint(ctx, sqlcgen.BindGoalCheckpointParams{
			SessionID: goalNullableString(req.SessionID), BindingHandle: goalNullableString(req.BindingHandle),
			BindingEpoch: goalNullableInt64(&req.BindingEpoch), UpdatedAt: store.FormatTimestamp(g.now().UTC()),
			LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation), NodeID: string(req.Key.NodeID),
			ItemIndex: int64(req.Key.ItemIndex), ControlEpoch: req.ExpectedControlEpoch,
		})
		if err != nil {
			return fmt.Errorf("store: bind Goal checkpoint: %w", err)
		}
		if err := requireGoalAffectedCount(affected, "bind Goal checkpoint"); err != nil {
			return err
		}
		updated, err = loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
		return err
	})
	return updated, err
}

// validateBindCheckpointRequest rejects incomplete task, session, phase, and epoch fences before writing.
func validateBindCheckpointRequest(req goal.BindCheckpointRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.ExpectedBindingEpoch < 0 || req.BindingEpoch < 1 ||
		!goalCheckpointPhaseValid(req.ExpectedPhase) || strings.TrimSpace(req.TaskRunID) == "" ||
		strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.BindingHandle) == "" {
		return fmt.Errorf("%w: Goal checkpoint binding identity is invalid", looppkg.ErrValidation)
	}
	return nil
}
