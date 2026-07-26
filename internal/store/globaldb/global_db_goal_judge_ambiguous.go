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

// MarkJudgeAmbiguous terminalizes both the evaluator attempt and its already-open work turn.
func (g *GoalRepo) MarkJudgeAmbiguous(ctx context.Context, req goal.MarkJudgeAmbiguousRequest) error {
	if err := g.checkReady(ctx, "mark goal judge ambiguous"); err != nil {
		return err
	}
	if err := validateMarkJudgeAmbiguousRequest(req); err != nil {
		return err
	}
	now := g.now()
	return g.withTaskImmediateTransaction(ctx, "mark goal judge ambiguous", func(exec taskSQLExecutor) error {
		return markJudgeAmbiguousWithExecutor(ctx, exec, req, now)
	})
}

func markJudgeAmbiguousWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.MarkJudgeAmbiguousRequest,
	now time.Time,
) error {
	if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
		return err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return err
	}
	attempt, err := getGoalJudgeAttemptWithExecutor(ctx, exec, req.Key, req.AttemptID)
	if err != nil {
		return err
	}
	if attempt.Status == goalJudgeStatusAmbiguous {
		return nil
	}
	if err := validateJudgeCheckpointOwner(checkpoint, req.ExpectedControlEpoch,
		req.ExpectedBindingEpoch, req.TaskRunID, req.PromptID); err != nil {
		return err
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		req.Key,
		checkpoint.BindingHandle,
		req.ExpectedBindingEpoch,
		checkpoint.SessionID,
	); err != nil {
		return err
	}
	if attempt.Status != goalJudgeStatusRunning {
		return goalControlStaleError("judge attempt is not running")
	}
	if err := markJudgeAttemptAmbiguous(ctx, exec, req, now); err != nil {
		return err
	}
	if err := finalizeAmbiguousJudgeTurn(ctx, exec, req, attempt.Turn, now); err != nil {
		return err
	}
	if err := checkpointAmbiguousJudge(ctx, exec, req, now); err != nil {
		return err
	}
	if err := projectGoalCheckpointCounts(
		ctx,
		exec,
		req.Key,
		goalStatusPaused,
		checkpoint.TurnsUsed,
		checkpoint.TurnLimit,
	); err != nil {
		return err
	}
	if err := appendGoalTurnCompletedEvent(ctx, exec, req.Key, req.PromptID, now); err != nil {
		return err
	}
	return appendGoalStatusChangedEvent(
		ctx,
		exec,
		req.Key,
		checkpoint.Status,
		goalStatusPaused,
		looppkg.ReasonCode(req.Cause),
		"system",
		"goal-recovery",
		now,
	)
}

func markJudgeAttemptAmbiguous(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.MarkJudgeAmbiguousRequest,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).MarkGoalJudgeAttemptAmbiguous(ctx, sqlcgen.MarkGoalJudgeAttemptAmbiguousParams{
		CompletedAt: store.FormatTimestamp(now), AttemptID: strings.TrimSpace(req.AttemptID),
		LoopRunID: string(req.Key.LoopRunID),
	})
	if err != nil {
		return fmt.Errorf("store: mark goal judge attempt ambiguous: %w", err)
	}
	return requireGoalAffectedCount(affected, "mark goal judge attempt ambiguous")
}

func finalizeAmbiguousJudgeTurn(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.MarkJudgeAmbiguousRequest,
	turn int,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).FinalizeAmbiguousGoalJudgeTurn(ctx, sqlcgen.FinalizeAmbiguousGoalJudgeTurnParams{
		ReasonCode: goalNullableString(req.Cause), EndedAt: store.FormatTimestamp(now),
		LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex), Turn: int64(turn),
	})
	if err != nil {
		return fmt.Errorf("store: finalize judge-ambiguous goal turn: %w", err)
	}
	return requireGoalAffectedCount(affected, "finalize judge-ambiguous goal turn")
}

func checkpointAmbiguousJudge(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.MarkJudgeAmbiguousRequest,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).CheckpointAmbiguousGoalJudge(ctx, sqlcgen.CheckpointAmbiguousGoalJudgeParams{
		ControlCause: goalNullableString(req.Cause), UpdatedAt: store.FormatTimestamp(now),
		LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
		JudgeAttemptID: goalNullableString(req.AttemptID),
	})
	if err != nil {
		return fmt.Errorf("store: checkpoint judge ambiguity: %w", err)
	}
	return requireGoalAffectedCount(affected, "checkpoint judge ambiguity")
}

func validateMarkJudgeAmbiguousRequest(req goal.MarkJudgeAmbiguousRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(req.AttemptID) == "" || req.ExpectedControlEpoch < 1 ||
		req.ExpectedBindingEpoch < 1 || strings.TrimSpace(req.TaskRunID) == "" ||
		strings.TrimSpace(req.PromptID) == "" ||
		(strings.TrimSpace(req.Cause) != string(looppkg.ReasonCodeGoalRecoveryAmbiguous) &&
			strings.TrimSpace(req.Cause) != string(looppkg.ReasonCodeGoalControlRevokedInFlight)) {
		return fmt.Errorf("%w: goal judge ambiguity identity is invalid", looppkg.ErrValidation)
	}
	return nil
}
