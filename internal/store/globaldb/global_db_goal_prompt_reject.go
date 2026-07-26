package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.PreparedPromptRejectionStore = (*GoalRepo)(nil)

// RejectPreparedGoalPrompt records a proven no-effect attempt and advances its prompt identity.
func (g *GoalRepo) RejectPreparedGoalPrompt(
	ctx context.Context,
	req goal.RejectPreparedPromptRequest,
) error {
	if err := g.checkReady(ctx, "reject prepared Goal prompt"); err != nil {
		return err
	}
	if err := validateRejectedGoalPromptRequest(req); err != nil {
		return err
	}
	now := g.now().UTC()
	return g.withTaskImmediateTransaction(ctx, "reject prepared Goal prompt", func(exec taskSQLExecutor) error {
		checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
		if err != nil {
			return err
		}
		row, err := loadGoalPromptRow(ctx, exec, req.Key.LoopRunID, req.PromptID)
		if err != nil {
			return err
		}
		if rejectedGoalPromptAlreadyApplied(checkpoint, &row, req) {
			return nil
		}
		if checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
			checkpoint.BindingEpoch != req.ExpectedBindingEpoch || checkpoint.Phase != goalCheckpointPhaseQueued ||
			checkpoint.TaskRunID != strings.TrimSpace(req.TaskRunID) ||
			checkpoint.QueueEntryID != strings.TrimSpace(req.QueueEntryID) ||
			checkpoint.PromptID != strings.TrimSpace(req.PromptID) {
			return goalControlStaleError("rejected Goal prompt checkpoint owner changed")
		}
		if err := requireGoalPromptOwner(
			&row,
			req.Key,
			req.TaskRunID,
			req.QueueEntryID,
			req.ExpectedBindingEpoch,
		); err != nil {
			return err
		}
		if row.status != store.SessionInputQueueStatusQueued || row.terminalAt != nil ||
			row.dispatchTokenHash.Valid {
			return goalControlStaleError("rejected Goal prompt already crossed submission")
		}
		if err := persistRejectedGoalQueueRow(ctx, exec, req, now); err != nil {
			return err
		}
		return advanceRejectedGoalCheckpoint(ctx, exec, req, checkpoint.PromptAttempt+1, now)
	})
}

func persistRejectedGoalQueueRow(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.RejectPreparedPromptRequest,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).RejectPreparedGoalQueueRow(ctx, sqlcgen.RejectPreparedGoalQueueRowParams{
		TerminalReasonCode: goalNullableString(string(req.ReasonCode)),
		TerminalAt:         store.FormatTimestamp(now),
		FailedAt: store.FormatTimestamp(
			now,
		),
		FailureSummary: string(req.ReasonCode),
		UpdatedAt:      store.FormatTimestamp(now),
		ID:             strings.TrimSpace(req.QueueEntryID),
		LoopRunID:      goalNullableString(string(req.Key.LoopRunID)),
		PromptID:       goalNullableString(req.PromptID),
		TaskRunID:      strings.TrimSpace(req.TaskRunID),
		OwnerEpoch: goalNullableInt64(
			&req.ExpectedControlEpoch,
		),
		BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
	})
	if err != nil {
		return fmt.Errorf("store: reject prepared Goal queue row: %w", err)
	}
	return requireGoalAffectedCount(affected, "reject prepared Goal queue row")
}

func advanceRejectedGoalCheckpoint(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.RejectPreparedPromptRequest,
	nextAttempt int,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).AdvanceRejectedGoalCheckpoint(ctx, sqlcgen.AdvanceRejectedGoalCheckpointParams{
		PromptAttempt: int64(nextAttempt), UpdatedAt: store.FormatTimestamp(now),
		LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
		TaskRunID: goalNullableString(req.TaskRunID), QueueEntryID: goalNullableString(req.QueueEntryID),
		PromptID: goalNullableString(req.PromptID),
	})
	if err != nil {
		return fmt.Errorf("store: advance rejected Goal checkpoint: %w", err)
	}
	return requireGoalAffectedCount(affected, "advance rejected Goal checkpoint")
}

func rejectedGoalPromptAlreadyApplied(
	checkpoint goal.Checkpoint,
	row *goalPromptRow,
	req goal.RejectPreparedPromptRequest,
) bool {
	return checkpoint.ControlEpoch == req.ExpectedControlEpoch &&
		checkpoint.BindingEpoch == req.ExpectedBindingEpoch && checkpoint.Phase == goalCheckpointPhasePreparing &&
		checkpoint.PromptAttempt == row.promptAttempt+1 && checkpoint.QueueEntryID == "" && checkpoint.PromptID == "" &&
		row.terminalAt != nil && row.terminalKind.Valid &&
		row.terminalKind.String == string(looppkg.ActionPromptOutcomeRejectedBeforeSubmit) &&
		row.terminalReason.Valid && row.terminalReason.String == string(req.ReasonCode)
}

func validateRejectedGoalPromptRequest(req goal.RejectPreparedPromptRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.ExpectedBindingEpoch < 1 ||
		strings.TrimSpace(req.TaskRunID) == "" || strings.TrimSpace(req.QueueEntryID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || strings.TrimSpace(string(req.ReasonCode)) == "" {
		return fmt.Errorf("%w: rejected Goal prompt identity is incomplete", looppkg.ErrValidation)
	}
	return nil
}
