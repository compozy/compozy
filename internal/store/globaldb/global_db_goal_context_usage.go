package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type goalContextUsageProjection struct {
	contextState     string
	usageSequence    int64
	pendingAfter     any
	baselineUsed     any
	recoveryRequired bool
	recoveryStreak   int
}

// RecordContextUsage advances the durable freshness cursor for one exact active binding.
func (g *GoalRepo) RecordContextUsage(
	ctx context.Context,
	req goal.RecordContextUsageRequest,
) (goal.Checkpoint, error) {
	if err := g.checkReady(ctx, "record Goal context usage"); err != nil {
		return goal.Checkpoint{}, err
	}
	if err := validateGoalContextUsageRequest(req); err != nil {
		return goal.Checkpoint{}, err
	}
	var updated goal.Checkpoint
	err := g.withTaskImmediateTransaction(ctx, "record Goal context usage", func(exec taskSQLExecutor) error {
		var err error
		updated, err = g.recordContextUsageWithExecutor(ctx, exec, req)
		return err
	})
	if err != nil {
		return goal.Checkpoint{}, err
	}
	return updated, nil
}

func (g *GoalRepo) recordContextUsageWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.RecordContextUsageRequest,
) (goal.Checkpoint, error) {
	if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
		return goal.Checkpoint{}, err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return goal.Checkpoint{}, err
	}
	if !goalContextUsageOwnerMatches(checkpoint, req) {
		return goal.Checkpoint{}, goalControlStaleError("Goal context usage owner changed")
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		req.Key,
		checkpoint.BindingHandle,
		req.ExpectedBindingEpoch,
		checkpoint.SessionID,
	); err != nil {
		return goal.Checkpoint{}, err
	}
	if !goalContextUsageAdvances(checkpoint, req.Usage.Sequence) {
		return checkpoint, nil
	}
	projection := goalContextUsageCheckpointProjection(checkpoint, req.Usage)
	now := g.now().UTC()
	usageSequence, err := goalSQLNullInt64(projection.usageSequence)
	if err != nil {
		return goal.Checkpoint{}, err
	}
	pendingAfter, err := goalSQLNullInt64(projection.pendingAfter)
	if err != nil {
		return goal.Checkpoint{}, err
	}
	baselineUsed, err := goalSQLNullInt64(projection.baselineUsed)
	if err != nil {
		return goal.Checkpoint{}, err
	}
	affected, err := sqlcgen.New(exec).UpdateGoalContextUsage(ctx, sqlcgen.UpdateGoalContextUsageParams{
		ContextState:               projection.contextState,
		UsageSequence:              usageSequence,
		UsagePendingAfterSequence:  pendingAfter,
		CompactionBaselineUsed:     baselineUsed,
		CompactionRecoveryRequired: int64(boolToInt(projection.recoveryRequired)),
		RecoveryStreak:             int64(projection.recoveryStreak),
		UpdatedAt:                  store.FormatTimestamp(now),
		LoopRunID:                  string(req.Key.LoopRunID),
		Generation:                 int64(req.Key.Generation),
		NodeID:                     string(req.Key.NodeID),
		ItemIndex:                  int64(req.Key.ItemIndex),
		ControlEpoch:               req.ExpectedControlEpoch,
		BindingEpoch:               sql.NullInt64{Int64: req.ExpectedBindingEpoch, Valid: true},
		Phase:                      strings.TrimSpace(req.ExpectedPhase),
		SessionID:                  goalNullableString(req.SessionID),
		BindingHandle:              goalNullableString(req.BindingHandle),
	})
	if err != nil {
		return goal.Checkpoint{}, fmt.Errorf("store: record Goal context usage: %w", err)
	}
	if affected != 1 {
		return goal.Checkpoint{}, goalControlStaleError("record Goal context usage lost compare-and-swap")
	}
	return loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
}

func goalContextUsageCheckpointProjection(
	checkpoint goal.Checkpoint,
	usage goal.ContextUsage,
) goalContextUsageProjection {
	projection := goalContextUsageProjection{
		contextState:     goalContextStateKnown,
		usageSequence:    usage.Sequence,
		recoveryRequired: checkpoint.CompactionRecoveryRequired,
		recoveryStreak:   checkpoint.RecoveryStreak,
	}
	if checkpoint.ContextState != goalContextStatePending {
		return projection
	}
	if checkpoint.CompactionBaselineUsed == nil || usage.Used < *checkpoint.CompactionBaselineUsed {
		projection.recoveryRequired = false
		projection.recoveryStreak = 0
		return projection
	}
	projection.recoveryRequired = true
	return projection
}

func goalContextUsageOwnerMatches(
	checkpoint goal.Checkpoint,
	req goal.RecordContextUsageRequest,
) bool {
	return checkpoint.ControlEpoch == req.ExpectedControlEpoch &&
		checkpoint.BindingEpoch == req.ExpectedBindingEpoch &&
		checkpoint.Phase == strings.TrimSpace(req.ExpectedPhase) &&
		checkpoint.Status == goalStatusActive &&
		checkpoint.SessionID == strings.TrimSpace(req.SessionID) &&
		checkpoint.BindingHandle == strings.TrimSpace(req.BindingHandle)
}

func goalContextUsageAdvances(checkpoint goal.Checkpoint, sequence int64) bool {
	switch checkpoint.ContextState {
	case goalContextStateKnown:
		return checkpoint.UsageSequence == nil || sequence > *checkpoint.UsageSequence
	case goalContextStateUnknown:
		return true
	case goalContextStatePending:
		return checkpoint.UsagePendingAfterSequence == nil || sequence > *checkpoint.UsagePendingAfterSequence
	default:
		return false
	}
}

func validateGoalContextUsageRequest(req goal.RecordContextUsageRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	usage := req.Usage
	if req.ExpectedControlEpoch < 1 || req.ExpectedBindingEpoch < 1 ||
		!goalCheckpointPhaseValid(strings.TrimSpace(req.ExpectedPhase)) ||
		strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.BindingHandle) == "" ||
		!usage.Known || usage.Used < 0 || usage.Size <= 0 || usage.Sequence < 0 || usage.ReportedAt.IsZero() {
		return fmt.Errorf("%w: Goal context usage observation is invalid", looppkg.ErrValidation)
	}
	return nil
}
