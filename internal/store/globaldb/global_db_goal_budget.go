package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const goalBudgetAuthorizationTTL = time.Second

var _ goal.BudgetGuard = (*GoalRepo)(nil)

// FlushAndCheck synchronously max-flushes one Goal task's cumulative usage and fences the next effect.
func (g *GoalRepo) FlushAndCheck(
	ctx context.Context,
	snapshot goal.BudgetBoundarySnapshot,
) (goal.BudgetDecision, error) {
	if err := g.checkReady(ctx, "flush and check Goal budget"); err != nil {
		return goal.BudgetDecision{}, err
	}
	if err := validateGoalBudgetSnapshot(snapshot); err != nil {
		return goal.BudgetDecision{}, err
	}
	var decision goal.BudgetDecision
	err := g.withTaskImmediateTransaction(ctx, "flush and check Goal budget", func(exec taskSQLExecutor) error {
		var err error
		decision, err = flushAndCheckGoalBudgetWithExecutor(ctx, exec, snapshot)
		return err
	})
	if err != nil {
		return goal.BudgetDecision{}, err
	}
	return decision, nil
}

func flushAndCheckGoalBudgetWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot goal.BudgetBoundarySnapshot,
) (goal.BudgetDecision, error) {
	if err := validateGoalRunWorkspace(ctx, exec, snapshot.Key); err != nil {
		return goal.BudgetDecision{}, err
	}
	now, err := goalBudgetDatabaseNow(ctx, exec)
	if err != nil {
		return goal.BudgetDecision{}, err
	}
	if err := flushGoalTaskUsage(ctx, exec, snapshot); err != nil {
		return goal.BudgetDecision{}, err
	}
	tokensUsed, err := refreshLoopTokensUsedWithExecutor(ctx, exec, string(snapshot.Key.LoopRunID))
	if err != nil {
		return goal.BudgetDecision{}, err
	}
	budgetTokens, budgetWallSec, onExceeded, startedAt, budgetVersion, err := advanceGoalBudgetVersion(
		ctx,
		exec,
		snapshot.Key.LoopRunID,
	)
	if err != nil {
		return goal.BudgetDecision{}, err
	}
	exceeded := goalBudgetExceeded(tokensUsed, budgetTokens, now, startedAt, budgetWallSec)
	grant, grantAllowed, err := goalBudgetGrantForBoundary(ctx, exec, snapshot, exceeded)
	if err != nil {
		return goal.BudgetDecision{}, err
	}
	decision := goal.BudgetDecision{
		Allowed:       !exceeded || grantAllowed,
		BudgetVersion: budgetVersion,
		ValidUntil:    goalBudgetValidUntil(now, startedAt, budgetWallSec, grantAllowed),
	}
	if grantAllowed {
		decision.Cause = grant.Cause
		decision.GrantID = grant.ID
		decision.GrantTurn = grant.Turn
		decision.GrantScope = grant.Scope
	}
	if decision.Allowed {
		return decision, nil
	}
	decision.Cause = loop.ReasonCodeGoalBudgetFenced
	decision.Disposition = loop.ActionDispositionExhausted
	if onExceeded == dsl.BudgetExceededEscalate {
		decision.Disposition = loop.ActionDispositionNeedsApproval
	}
	return decision, nil
}

func goalBudgetDatabaseNow(ctx context.Context, exec taskSQLExecutor) (time.Time, error) {
	raw, err := sqlcgen.New(exec).GetGoalDatabaseNow(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: read Goal budget database time: %w", err)
	}
	now, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse Goal budget database time: %w", err)
	}
	return now.UTC(), nil
}

func flushGoalTaskUsage(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot goal.BudgetBoundarySnapshot,
) error {
	affected, err := sqlcgen.New(exec).FlushGoalTaskUsage(ctx, sqlcgen.FlushGoalTaskUsageParams{
		TokensReported: boolToInt(snapshot.TokensReported), LiveTokensUsed: snapshot.LiveTokensUsed,
		ID: strings.TrimSpace(snapshot.TaskRunID), TaskLoopRunID: goalRequiredSQLString(string(snapshot.Key.LoopRunID)),
		CheckpointLoopRunID: string(snapshot.Key.LoopRunID), Generation: int64(snapshot.Key.Generation),
		NodeID: string(snapshot.Key.NodeID), ItemIndex: int64(snapshot.Key.ItemIndex),
		ControlEpoch: snapshot.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&snapshot.ExpectedBindingEpoch),
		Phase: strings.TrimSpace(snapshot.ExpectedPhase), TaskRunID: goalRequiredSQLString(snapshot.TaskRunID),
		QueueEntryID: goalRequiredSQLString(snapshot.ExpectedQueueEntryID),
		PromptID:     goalRequiredSQLString(snapshot.ExpectedPromptID),
	})
	if err != nil {
		return fmt.Errorf("store: flush Goal task usage: %w", err)
	}
	return requireGoalAffectedCount(affected, "flush Goal task usage")
}

func advanceGoalBudgetVersion(
	ctx context.Context,
	exec taskSQLExecutor,
	runID loop.RunID,
) (int, int, dsl.BudgetExceeded, time.Time, int64, error) {
	row, err := sqlcgen.New(exec).AdvanceGoalBudgetVersion(ctx, string(runID))
	if err != nil {
		return 0, 0, "", time.Time{}, 0, fmt.Errorf("store: advance Goal budget version: %w", err)
	}
	startedAt, err := store.ParseTimestamp(row.StartedAt)
	if err != nil {
		return 0, 0, "", time.Time{}, 0, fmt.Errorf("store: parse Goal budget started_at: %w", err)
	}
	return int(row.BudgetTokens),
		int(row.BudgetWallSec),
		dsl.BudgetExceeded(strings.TrimSpace(row.BudgetOnExceeded)),
		startedAt,
		row.BudgetVersion,
		nil
}

func validateGoalBudgetSnapshot(snapshot goal.BudgetBoundarySnapshot) error {
	if err := snapshot.Key.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(snapshot.TaskRunID) == "" || snapshot.ExpectedControlEpoch < 1 ||
		snapshot.ExpectedBindingEpoch < 0 || !goalCheckpointPhaseValid(strings.TrimSpace(snapshot.ExpectedPhase)) ||
		strings.TrimSpace(string(snapshot.Boundary)) == "" ||
		strings.TrimSpace(snapshot.Phase) == "" || snapshot.Turn < 0 ||
		strings.TrimSpace(snapshot.OperationID) == "" || snapshot.OperationBaseTokens < 0 ||
		snapshot.LiveTokensUsed < 0 || snapshot.LiveTokensUsed < snapshot.OperationBaseTokens {
		return fmt.Errorf("%w: Goal budget boundary snapshot is invalid", loop.ErrValidation)
	}
	return nil
}

func goalBudgetExceeded(
	tokensUsed int64,
	budgetTokens int,
	now time.Time,
	startedAt time.Time,
	budgetWallSec int,
) bool {
	if budgetTokens > 0 && tokensUsed >= int64(budgetTokens) {
		return true
	}
	return budgetWallSec > 0 && !now.Before(startedAt.Add(time.Duration(budgetWallSec)*time.Second))
}

func goalBudgetValidUntil(now time.Time, startedAt time.Time, wallSeconds int, approved bool) time.Time {
	validUntil := now.Add(goalBudgetAuthorizationTTL)
	if approved || wallSeconds <= 0 {
		return validUntil
	}
	wallDeadline := startedAt.Add(time.Duration(wallSeconds) * time.Second)
	if wallDeadline.Before(validUntil) {
		return wallDeadline
	}
	return validUntil
}

func goalBudgetGrantForBoundary(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot goal.BudgetBoundarySnapshot,
	exceeded bool,
) (goal.ControlGrant, bool, error) {
	if !exceeded {
		return goal.ControlGrant{}, false, nil
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, snapshot.Key)
	if err != nil {
		return goal.ControlGrant{}, false, err
	}
	if checkpoint.ControlGrant == nil || checkpoint.ControlGrant.Consumed ||
		checkpoint.ControlGrant.Kind != goal.ControlGrantBudget ||
		checkpoint.ControlGrant.Turn != int(snapshot.Turn) {
		return goal.ControlGrant{}, false, nil
	}
	grant := *checkpoint.ControlGrant
	allowed := false
	switch snapshot.Boundary {
	case goal.BudgetBeforeWork:
		allowed = grant.Scope == goal.ControlGrantScopeWorkAndSettle
	case goal.BudgetAfterWork, goal.BudgetBeforeCompact, goal.BudgetAfterCompact,
		goal.BudgetBeforeJudge, goal.BudgetAfterJudge:
		allowed = grant.Scope == goal.ControlGrantScopeSettleCurrent ||
			grant.Scope == goal.ControlGrantScopeWorkAndSettle
	}
	return grant, allowed, nil
}

func validateGoalBudgetDecision(
	ctx context.Context,
	exec taskSQLExecutor,
	runID loop.RunID,
	decision goal.BudgetDecision,
) error {
	if !decision.Allowed || decision.BudgetVersion < 0 || decision.ValidUntil.IsZero() {
		return goalPromptFencedError("budget decision does not authorize an effect")
	}
	valid, err := sqlcgen.New(exec).ValidateGoalBudgetDecision(ctx, sqlcgen.ValidateGoalBudgetDecisionParams{
		BudgetVersion: decision.BudgetVersion,
		ValidUntil:    store.FormatTimestamp(decision.ValidUntil),
		ID:            string(runID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", loop.ErrRunNotFound, runID)
	}
	if err != nil {
		return fmt.Errorf("store: validate goal budget decision: %w", err)
	}
	if valid != 1 {
		return goalPromptFencedError("budget decision version or validity window is stale")
	}
	return nil
}
