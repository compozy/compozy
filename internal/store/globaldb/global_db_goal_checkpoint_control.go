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

// GrantAndReactivate records one monotonic typed grant and advances the control epoch exactly once.
func (g *GoalRepo) GrantAndReactivate(ctx context.Context, req goal.GrantRequest) error {
	if err := g.checkReady(ctx, "grant and reactivate goal checkpoint"); err != nil {
		return err
	}
	if err := validateGoalGrantRequest(req); err != nil {
		return err
	}
	now := g.now()
	return g.withTaskImmediateTransaction(
		ctx,
		"grant and reactivate goal checkpoint",
		func(exec taskSQLExecutor) error {
			return grantAndReactivateWithExecutor(ctx, exec, req, now)
		},
	)
}

func grantAndReactivateWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.GrantRequest,
	now time.Time,
) error {
	if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
		return err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return err
	}
	if checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
		checkpoint.Phase != goalCheckpointPhaseAwaitingControl {
		return goalControlStaleError("grant checkpoint epoch or phase changed")
	}
	grantID := int64(1)
	if checkpoint.ControlGrant != nil {
		grantID = checkpoint.ControlGrant.ID + 1
	}
	consumed := req.Kind == goal.ControlGrantTurnExtension || req.Kind == goal.ControlGrantPlainResume
	turnLimit := checkpoint.TurnLimit
	if req.Kind == goal.ControlGrantTurnExtension {
		turnLimit += req.TurnIncrement
	}
	if err := persistGoalGrantCheckpoint(ctx, exec, req, grantID, consumed, turnLimit, now); err != nil {
		return err
	}
	if err := incrementGoalBudgetVersion(ctx, exec, req.Key.LoopRunID); err != nil {
		return err
	}
	if err := projectGoalGrantOutput(ctx, exec, req, checkpoint, turnLimit); err != nil {
		return err
	}
	return appendGoalStatusChangedEvent(
		ctx,
		exec,
		req.Key,
		checkpoint.Status,
		goalStatusActive,
		req.Cause,
		req.ActorKind,
		req.ActorID,
		now,
	)
}

func persistGoalGrantCheckpoint(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.GrantRequest,
	grantID int64,
	consumed bool,
	turnLimit int,
	now time.Time,
) error {
	grantTurn := int64(req.Turn)
	affected, err := sqlcgen.New(exec).GrantGoalCheckpointControl(ctx, sqlcgen.GrantGoalCheckpointControlParams{
		TurnLimit: int64(turnLimit), ControlGrantID: grantID, ControlGrantKind: goalNullableString(string(req.Kind)),
		ControlGrantCause: goalNullableString(string(req.Cause)), ControlGrantTurn: goalNullableInt64(&grantTurn),
		ControlGrantScope: goalNullableString(string(req.Scope)), ControlGrantConsumed: int64(boolToInt(consumed)),
		UpdatedAt: store.FormatTimestamp(now), LoopRunID: string(req.Key.LoopRunID),
		Generation: int64(req.Key.Generation), NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		ControlEpoch: req.ExpectedControlEpoch,
	})
	if err != nil {
		return fmt.Errorf("store: grant goal checkpoint control: %w", err)
	}
	return requireGoalAffectedCount(affected, "grant goal checkpoint control")
}

func projectGoalGrantOutput(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.GrantRequest,
	checkpoint goal.Checkpoint,
	turnLimit int,
) error {
	turnsUsed64, turnLimit64 := int64(checkpoint.TurnsUsed), int64(turnLimit)
	affected, err := sqlcgen.New(exec).ProjectGoalGrantOutput(ctx, sqlcgen.ProjectGoalGrantOutputParams{
		GoalTurnsUsed: goalNullableInt64(&turnsUsed64), GoalTurnLimit: goalNullableInt64(&turnLimit64),
		LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
	})
	if err != nil {
		return fmt.Errorf("store: project goal grant reactivation: %w", err)
	}
	return requireGoalAffectedCount(affected, "project goal grant reactivation")
}

// CheckpointControl records one typed nonterminal or terminal control boundary by CAS.
func (g *GoalRepo) CheckpointControl(ctx context.Context, req goal.ControlCheckpointRequest) error {
	if err := g.checkReady(ctx, "checkpoint goal control"); err != nil {
		return err
	}
	if err := validateGoalControlCheckpointRequest(req); err != nil {
		return err
	}
	now := g.now()
	return g.withTaskImmediateTransaction(ctx, "checkpoint goal control", func(exec taskSQLExecutor) error {
		if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
			return err
		}
		checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
		if err != nil {
			return err
		}
		if !goalControlCheckpointOwnerMatches(checkpoint, req) {
			return goalControlStaleError("control checkpoint owner changed")
		}
		phase := goalControlCheckpointTargetPhase(req)
		affected, err := sqlcgen.New(exec).UpdateGoalCheckpointControl(ctx, sqlcgen.UpdateGoalCheckpointControlParams{
			Phase: phase, GoalStatus: req.Status, ControlCause: goalNullableString(string(req.Cause)),
			ControlActorKind: goalNullableString(req.ActorKind), ControlActorID: goalNullableString(req.ActorID),
			ControlRequestedAt: store.FormatTimestamp(now), UpdatedAt: store.FormatTimestamp(now),
			LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
			NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
			ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
			ExpectedPhase: strings.TrimSpace(req.ExpectedPhase), TaskRunID: goalRequiredSQLString(req.TaskRunID),
			QueueEntryID: goalRequiredSQLString(req.QueueEntryID), PromptID: goalRequiredSQLString(req.PromptID),
		})
		if err != nil {
			return fmt.Errorf("store: checkpoint goal control: %w", err)
		}
		if err := requireGoalAffectedCount(affected, "checkpoint goal control"); err != nil {
			return err
		}
		if phase == goalCheckpointPhaseTerminal {
			if err := closeTerminalGoalCheckpointBinding(ctx, exec, checkpoint, now); err != nil {
				return err
			}
		}
		return appendGoalStatusChangedEvent(
			ctx,
			exec,
			req.Key,
			checkpoint.Status,
			req.Status,
			req.Cause,
			req.ActorKind,
			req.ActorID,
			now,
		)
	})
}

func validateGoalGrantRequest(req goal.GrantRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.Kind == "" || req.Cause == "" || req.Scope == "" ||
		req.Turn < 0 || strings.TrimSpace(req.ActorKind) == "" || strings.TrimSpace(req.ActorID) == "" {
		return fmt.Errorf("%w: goal grant identity is incomplete", looppkg.ErrValidation)
	}
	if req.Kind == goal.ControlGrantTurnExtension && req.TurnIncrement < 1 {
		return fmt.Errorf("%w: turn extension must increase the limit", looppkg.ErrValidation)
	}
	if req.Kind != goal.ControlGrantTurnExtension && req.TurnIncrement != 0 {
		return fmt.Errorf("%w: only turn-extension may change the turn limit", looppkg.ErrValidation)
	}
	if !goalGrantKindScopeValid(req.Kind, req.Scope) {
		return fmt.Errorf(
			"%w: goal grant kind %q cannot use scope %q",
			looppkg.ErrValidation,
			req.Kind,
			req.Scope,
		)
	}
	return nil
}

func goalGrantKindScopeValid(kind goal.ControlGrantKind, scope goal.ControlGrantScope) bool {
	switch kind {
	case goal.ControlGrantTurnExtension:
		return scope == goal.ControlGrantScopeTurnLimit
	case goal.ControlGrantBudget:
		return scope == goal.ControlGrantScopeSettleCurrent ||
			scope == goal.ControlGrantScopeWorkAndSettle
	case goal.ControlGrantReseed:
		return scope == goal.ControlGrantScopeRotateBinding
	case goal.ControlGrantPlainResume:
		return scope == goal.ControlGrantScopeReactivate
	default:
		return false
	}
}

func incrementGoalBudgetVersion(ctx context.Context, exec taskSQLExecutor, runID looppkg.RunID) error {
	affected, err := sqlcgen.New(exec).IncrementGoalBudgetVersion(ctx, string(runID))
	if err != nil {
		return fmt.Errorf("store: increment goal budget version: %w", err)
	}
	return requireGoalAffectedCount(affected, "increment goal budget version")
}
