package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func validateClaimPreparedWorkRequest(req goal.ClaimPreparedWorkPromptRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.BindingEpoch < 1 ||
		strings.TrimSpace(req.TaskRunID) == "" || strings.TrimSpace(req.QueueEntryID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.BindingHandle) == "" || req.UsageBaseTokens < 0 ||
		(!req.UsageBaseReported && req.UsageBaseTokens != 0) ||
		strings.TrimSpace(req.ActorKind) == "" || strings.TrimSpace(req.ActorID) == "" {
		return fmt.Errorf("%w: Goal work claim identity is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func validateClaimPreparedCompactionRequest(req goal.ClaimPreparedCompactionRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.BindingEpoch < 1 ||
		strings.TrimSpace(req.TaskRunID) == "" || strings.TrimSpace(req.QueueEntryID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.BindingHandle) == "" || req.UsageBaseTokens < 0 ||
		(!req.UsageBaseReported && req.UsageBaseTokens != 0) ||
		(req.UsageSequence != nil && *req.UsageSequence < 0) {
		return fmt.Errorf("%w: Goal compaction claim identity is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func validateGoalDecisionGrant(
	checkpoint goal.Checkpoint,
	decision goal.BudgetDecision,
	turn int,
	work bool,
) error {
	if decision.GrantID == 0 {
		return nil
	}
	grant := checkpoint.ControlGrant
	if grant == nil || grant.Consumed || grant.Kind != goal.ControlGrantBudget ||
		grant.ID != decision.GrantID || grant.Turn != turn || grant.Turn != decision.GrantTurn ||
		grant.Scope != decision.GrantScope || grant.Cause != decision.Cause {
		return goalPromptFencedError("Goal budget grant identity changed")
	}
	if work && grant.Scope != goal.ControlGrantScopeWorkAndSettle {
		return goalPromptFencedError("settle-current grant cannot start Goal work")
	}
	if !work && grant.Scope != goal.ControlGrantScopeSettleCurrent &&
		grant.Scope != goal.ControlGrantScopeWorkAndSettle {
		return goalPromptFencedError("Goal grant does not authorize compaction")
	}
	return nil
}

func projectGoalCheckpointCounts(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	status string,
	turnsUsed int,
	turnLimit int,
) error {
	err := sqlcgen.New(exec).ProjectGoalCheckpointCounts(ctx, sqlcgen.ProjectGoalCheckpointCountsParams{
		GoalStatus: goalNullableString(status), GoalTurnsUsed: sql.NullInt64{Int64: int64(turnsUsed), Valid: true},
		GoalTurnLimit: sql.NullInt64{Int64: int64(turnLimit), Valid: true}, LoopRunID: string(key.LoopRunID),
		Generation: int64(key.Generation), NodeID: string(key.NodeID), ItemIndex: int64(key.ItemIndex),
	})
	if err != nil {
		return fmt.Errorf("store: project Goal checkpoint counts: %w", err)
	}
	return nil
}

func nullableGoalInt64Value(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
