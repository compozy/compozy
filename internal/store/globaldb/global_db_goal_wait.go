package globaldb

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ goal.WaitStore = (*GoalRepo)(nil)

// SetWaitState preserves the settled turn and serializes wait transitions with controls.
func (g *GoalRepo) SetWaitState(ctx context.Context, req goal.WaitCheckpointRequest) (goal.Checkpoint, error) {
	if err := g.checkReady(ctx, "set Goal wait state"); err != nil {
		return goal.Checkpoint{}, err
	}
	if err := req.Checkpoint.Key.Validate(); err != nil {
		return goal.Checkpoint{}, err
	}
	var updated goal.Checkpoint
	err := g.withTaskImmediateTransaction(ctx, "set Goal wait state", func(exec taskSQLExecutor) error {
		if err := validateGoalRunWorkspace(ctx, exec, req.Checkpoint.Key); err != nil {
			return err
		}
		current, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Checkpoint.Key)
		if err != nil {
			return err
		}
		owner := req.Checkpoint
		if !goalWaitOwnerMatches(current, owner) {
			return goalControlStaleError("Goal wait owner changed")
		}
		updated = current
		if current.ControlActorKind != "" || current.ControlActorID != "" {
			return nil // The executor must settle the admitted pause first.
		}
		phase, status := goalCheckpointPhaseIdle, goalStatusActive
		startedAt := current.WaitStartedAt
		now := g.now().UTC()
		if req.Waiting {
			phase, status = goalCheckpointPhaseWaiting, goalStatusWaiting
			if current.Phase != goalCheckpointPhaseWaiting {
				startedAt = &now
			}
		} else {
			startedAt = nil
		}
		if current.Phase == phase {
			return nil
		}
		if err := persistGoalWaitTransition(ctx, exec, current, phase, status, startedAt, now); err != nil {
			return err
		}
		updated, err = loadGoalCheckpointWithExecutor(ctx, exec, owner.Key)
		return err
	})
	return updated, err
}

func goalWaitOwnerMatches(current, owner goal.Checkpoint) bool {
	return current.ControlEpoch == owner.ControlEpoch && current.BindingEpoch == owner.BindingEpoch &&
		current.SessionID == owner.SessionID && current.TaskRunID == owner.TaskRunID &&
		current.Phase == owner.Phase && current.PromptID == "" && current.QueueEntryID == "" &&
		(current.Phase == goalCheckpointPhaseIdle || current.Phase == goalCheckpointPhaseWaiting)
}

func persistGoalWaitTransition(ctx context.Context, exec taskSQLExecutor, current goal.Checkpoint,
	phase, status string, startedAt *time.Time, now time.Time) error {
	affected, err := sqlcgen.New(exec).SetGoalCheckpointWait(ctx, sqlcgen.SetGoalCheckpointWaitParams{
		Phase:         phase,
		GoalStatus:    status,
		WaitStartedAt: goalNullableTimestampString(startedAt),
		UpdatedAt:     store.FormatTimestamp(now),
		LoopRunID:     string(current.Key.LoopRunID),
		Generation: int64(
			current.Key.Generation,
		),
		NodeID:        string(current.Key.NodeID),
		ItemIndex:     int64(current.Key.ItemIndex),
		ControlEpoch:  current.ControlEpoch,
		BindingEpoch:  goalNullableInt64(&current.BindingEpoch),
		SessionID:     goalNullableString(current.SessionID),
		TaskRunID:     goalNullableString(current.TaskRunID),
		ExpectedPhase: current.Phase,
	})
	if err != nil {
		return fmt.Errorf("store: set Goal wait state: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "set Goal wait state"); err != nil {
		return err
	}
	if err := projectGoalCheckpointCounts(
		ctx,
		exec,
		current.Key,
		status,
		current.TurnsUsed,
		current.TurnLimit,
	); err != nil {
		return err
	}
	if err := appendGoalStatusChangedEvent(
		ctx,
		exec,
		current.Key,
		current.Status,
		status,
		"",
		"system",
		"goal-executor",
		now,
	); err != nil {
		return err
	}
	return nil
}
