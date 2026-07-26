package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.CheckpointStore = (*GoalRepo)(nil)

// CreateCheckpoint inserts one initial Goal checkpoint without overwriting durable progress.
func (g *GoalRepo) CreateCheckpoint(
	ctx context.Context,
	req goal.CreateCheckpointRequest,
) (goal.Checkpoint, error) {
	if err := g.checkReady(ctx, "create goal checkpoint"); err != nil {
		return goal.Checkpoint{}, err
	}
	checkpoint := req.Checkpoint
	if checkpoint.ControlEpoch == 0 {
		checkpoint.ControlEpoch = 1
	}
	if checkpoint.UpdatedAt.IsZero() {
		checkpoint.UpdatedAt = g.now()
	}
	if err := validateGoalCheckpoint(checkpoint); err != nil {
		return goal.Checkpoint{}, err
	}
	var persisted goal.Checkpoint
	err := g.withTaskImmediateTransaction(ctx, "create goal checkpoint", func(exec taskSQLExecutor) error {
		if err := validateGoalRunWorkspace(ctx, exec, checkpoint.Key); err != nil {
			return err
		}
		statusBeforePause := checkpoint.Status
		if err := projectPendingPauseToNewGoalCheckpoint(ctx, exec, &checkpoint); err != nil {
			return err
		}
		affected, err := sqlcgen.New(exec).InsertGoalCheckpoint(ctx, goalCheckpointInsertParams(checkpoint))
		if err != nil {
			return fmt.Errorf("store: insert goal checkpoint: %w", err)
		}
		persisted, err = loadGoalCheckpointWithExecutor(ctx, exec, checkpoint.Key)
		if err != nil {
			return err
		}
		if persisted.TurnLimit != checkpoint.TurnLimit ||
			persisted.ContextNudgeRatio != checkpoint.ContextNudgeRatio {
			return goalControlStaleError("checkpoint already exists with different pinned policy")
		}
		if affected == 1 && statusBeforePause != checkpoint.Status {
			return appendGoalStatusChangedEvent(
				ctx,
				exec,
				checkpoint.Key,
				statusBeforePause,
				checkpoint.Status,
				checkpoint.ControlCause,
				checkpoint.ControlActorKind,
				checkpoint.ControlActorID,
				checkpoint.UpdatedAt,
			)
		}
		return nil
	})
	if err != nil {
		return goal.Checkpoint{}, err
	}
	return persisted, nil
}

func projectPendingPauseToNewGoalCheckpoint(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint *goal.Checkpoint,
) error {
	if checkpoint == nil || checkpoint.Phase == goalCheckpointPhaseAwaitingControl ||
		checkpoint.Phase == goalCheckpointPhaseTerminal {
		return nil
	}
	pauseRequested, actorKind, actorID, requestedAt, err := pendingGoalPauseActor(
		ctx,
		exec,
		checkpoint.Key.LoopRunID,
	)
	if err != nil {
		return err
	}
	if !pauseRequested {
		return nil
	}
	checkpoint.ControlActorKind = actorKind
	checkpoint.ControlActorID = actorID
	checkpoint.ControlRequestedAt = &requestedAt
	checkpoint.Phase = goalCheckpointPhaseAwaitingControl
	checkpoint.Status = goalStatusPaused
	checkpoint.ControlCause = loop.ReasonCode(loop.TransitionCausePauseBoundary)
	return validateGoalCheckpointControl(*checkpoint)
}

// LoadCheckpoint returns one checkpoint only through its owning workspace.
func (g *GoalRepo) LoadCheckpoint(ctx context.Context, key goal.TurnKey) (goal.Checkpoint, error) {
	if err := g.checkReady(ctx, "load goal checkpoint"); err != nil {
		return goal.Checkpoint{}, err
	}
	if err := validateGoalRunWorkspace(ctx, g.db, key); err != nil {
		return goal.Checkpoint{}, err
	}
	return loadGoalCheckpointWithExecutor(ctx, g.db, key)
}

func loadGoalCheckpointWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
) (goal.Checkpoint, error) {
	row, err := sqlcgen.New(exec).GetGoalCheckpoint(ctx, sqlcgen.GetGoalCheckpointParams{
		LoopRunID: string(key.LoopRunID), Generation: int64(key.Generation),
		NodeID: string(key.NodeID), ItemIndex: int64(key.ItemIndex),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goal.Checkpoint{}, fmt.Errorf("%w: %s/%d/%s/%d", goal.ErrCheckpointNotFound,
			key.LoopRunID, key.Generation, key.NodeID, key.ItemIndex)
	}
	if err != nil {
		return goal.Checkpoint{}, fmt.Errorf("store: scan goal checkpoint: %w", err)
	}
	checkpoint, err := goalCheckpointFromGenerated(&row, key.WorkspaceID)
	if err != nil {
		return goal.Checkpoint{}, fmt.Errorf("store: decode goal checkpoint: %w", err)
	}
	return checkpoint, nil
}

func validateGoalCheckpoint(checkpoint goal.Checkpoint) error {
	if err := checkpoint.Key.Validate(); err != nil {
		return err
	}
	if checkpoint.ControlEpoch < 1 || checkpoint.TurnLimit < 1 || checkpoint.TurnsUsed < 0 ||
		checkpoint.BrokenStreak < 0 || checkpoint.RecoveryStreak < 0 {
		return fmt.Errorf("%w: goal checkpoint counters are invalid", loop.ErrValidation)
	}
	if !goalCheckpointPhaseValid(checkpoint.Phase) || !goalStatusValid(checkpoint.Status) {
		return fmt.Errorf("%w: goal checkpoint phase or status is invalid", loop.ErrValidation)
	}
	if checkpoint.ContextNudgeRatio < 0 || checkpoint.ContextNudgeRatio > 1 {
		return fmt.Errorf("%w: goal context_nudge_ratio must be in [0,1]", loop.ErrValidation)
	}
	if err := validateGoalContextFreshness(checkpoint); err != nil {
		return err
	}
	if err := validateGoalCheckpointPrompt(checkpoint); err != nil {
		return err
	}
	if err := validateGoalCheckpointControl(checkpoint); err != nil {
		return err
	}
	return nil
}

func goalCheckpointPhaseValid(phase string) bool {
	switch strings.TrimSpace(phase) {
	case goalCheckpointPhaseIdle,
		goalCheckpointPhasePreparing,
		goalCheckpointPhaseQueued,
		goalCheckpointPhasePrompting,
		goalCheckpointPhaseCompacting,
		goalCheckpointPhaseJudging,
		goalCheckpointPhasePersisting,
		goalCheckpointPhaseAwaitingControl,
		goalCheckpointPhaseTerminal:
		return true
	default:
		return false
	}
}

func goalStatusValid(status string) bool {
	switch strings.TrimSpace(status) {
	case goalStatusActive,
		goalStatusPaused,
		goalStatusBlocked,
		goalStatusUsageLimited,
		goalStatusBudgetLimited,
		goalStatusComplete:
		return true
	default:
		return false
	}
}

func validateGoalContextFreshness(checkpoint goal.Checkpoint) error {
	if err := validateGoalCompactionFreshness(checkpoint); err != nil {
		return err
	}
	switch checkpoint.ContextState {
	case goalContextStateKnown:
		if checkpoint.UsageSequence == nil || checkpoint.UsagePendingAfterSequence != nil {
			return fmt.Errorf("%w: known goal context requires usage sequence only", loop.ErrValidation)
		}
	case goalContextStateUnknown:
		if checkpoint.UsageSequence != nil || checkpoint.UsagePendingAfterSequence != nil {
			return fmt.Errorf("%w: unknown goal context cannot carry usage sequence", loop.ErrValidation)
		}
	case goalContextStatePending:
		if checkpoint.UsagePendingAfterSequence != nil && checkpoint.UsageSequence != nil &&
			*checkpoint.UsagePendingAfterSequence != *checkpoint.UsageSequence {
			return fmt.Errorf("%w: pending goal context freshness floor is invalid", loop.ErrValidation)
		}
	default:
		return fmt.Errorf("%w: goal context state is invalid", loop.ErrValidation)
	}
	return nil
}

func validateGoalCompactionFreshness(checkpoint goal.Checkpoint) error {
	if checkpoint.CompactionBaselineUsed != nil && *checkpoint.CompactionBaselineUsed < 0 {
		return fmt.Errorf("%w: Goal compaction baseline usage is invalid", loop.ErrValidation)
	}
	if checkpoint.CompactionRecoveryRequired && checkpoint.ContextState != goalContextStateKnown {
		return fmt.Errorf("%w: Goal compaction recovery requires known usage", loop.ErrValidation)
	}
	baselineAllowed := checkpoint.ContextState == goalContextStatePending ||
		(checkpoint.ContextState == goalContextStateKnown && checkpoint.PromptKind == goalPromptKindCompact &&
			(checkpoint.Phase == goalCheckpointPhaseQueued || checkpoint.Phase == goalCheckpointPhaseCompacting))
	if checkpoint.CompactionBaselineUsed != nil && !baselineAllowed {
		return fmt.Errorf("%w: Goal compaction baseline has no active compact operation", loop.ErrValidation)
	}
	return nil
}

func validateGoalCheckpointPrompt(checkpoint goal.Checkpoint) error {
	if checkpoint.PromptKind == "" && checkpoint.PromptID == "" && checkpoint.QueueEntryID == "" {
		return nil
	}
	if checkpoint.PromptKind != goalPromptKindWork && checkpoint.PromptKind != goalPromptKindContinuation &&
		checkpoint.PromptKind != goalPromptKindCompact {
		return fmt.Errorf("%w: goal checkpoint prompt kind is invalid", loop.ErrValidation)
	}
	if strings.TrimSpace(checkpoint.PromptID) == "" || strings.TrimSpace(checkpoint.QueueEntryID) == "" ||
		strings.TrimSpace(checkpoint.TaskRunID) == "" || strings.TrimSpace(checkpoint.SessionID) == "" ||
		strings.TrimSpace(checkpoint.BindingHandle) == "" || checkpoint.BindingEpoch < 1 {
		return fmt.Errorf("%w: goal checkpoint prompt identity is incomplete", loop.ErrValidation)
	}
	return nil
}

func validateGoalCheckpointControl(checkpoint goal.Checkpoint) error {
	hasActor := checkpoint.ControlActorKind != "" || checkpoint.ControlActorID != "" ||
		checkpoint.ControlRequestedAt != nil
	if hasActor && (strings.TrimSpace(checkpoint.ControlActorKind) == "" ||
		strings.TrimSpace(checkpoint.ControlActorID) == "" || checkpoint.ControlRequestedAt == nil) {
		return fmt.Errorf("%w: goal control actor identity is incomplete", loop.ErrValidation)
	}
	if checkpoint.ControlGrant != nil && (checkpoint.ControlGrant.ID < 1 ||
		checkpoint.ControlGrant.Kind == "" || checkpoint.ControlGrant.Cause == "" ||
		checkpoint.ControlGrant.Scope == "" || checkpoint.ControlGrant.Turn < 0) {
		return fmt.Errorf("%w: goal control grant identity is incomplete", loop.ErrValidation)
	}
	return nil
}
