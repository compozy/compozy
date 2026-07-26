package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.JudgeStore = (*GoalRepo)(nil)

// BeginJudgeAttempt persists one aggregate judge attempt before evaluator effects.
func (g *GoalRepo) BeginJudgeAttempt(
	ctx context.Context,
	req goal.BeginJudgeAttemptRequest,
) (goal.JudgeAttempt, error) {
	if err := g.checkReady(ctx, "begin goal judge attempt"); err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := validateBeginJudgeAttemptRequest(req); err != nil {
		return goal.JudgeAttempt{}, err
	}
	now := g.now()
	var attempt goal.JudgeAttempt
	err := g.withTaskImmediateTransaction(ctx, "begin goal judge attempt", func(exec taskSQLExecutor) error {
		var err error
		attempt, err = beginJudgeAttemptWithExecutor(ctx, exec, req, now)
		return err
	})
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	return attempt, nil
}

func beginJudgeAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.BeginJudgeAttemptRequest,
	now time.Time,
) (goal.JudgeAttempt, error) {
	if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
		return goal.JudgeAttempt{}, err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	if strings.TrimSpace(checkpoint.ControlActorKind) != "" ||
		strings.TrimSpace(checkpoint.ControlActorID) != "" {
		return goal.JudgeAttempt{}, goalPromptFencedError("Goal pause intent won before judge start")
	}
	if err := validateJudgeCheckpointOwner(checkpoint, req.ExpectedControlEpoch,
		req.ExpectedBindingEpoch, req.TaskRunID, req.PromptID); err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		req.Key,
		checkpoint.BindingHandle,
		req.ExpectedBindingEpoch,
		checkpoint.SessionID,
	); err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := validateGoalBudgetDecision(ctx, exec, req.Key.LoopRunID, req.BudgetDecision); err != nil {
		return goal.JudgeAttempt{}, err
	}
	existing, found, digest, err := findGoalJudgeAttemptWithDigest(ctx, exec, req.Key, req.AttemptID)
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	if found {
		if digest != strings.TrimSpace(req.JudgeDigest) || existing.Turn != req.Turn ||
			existing.UsageBaseTokens != req.UsageBaseTokens {
			return goal.JudgeAttempt{}, goalControlStaleError(
				"judge attempt identity differs from persisted attempt",
			)
		}
		return existing, nil
	}
	if err := insertGoalJudgeAttempt(ctx, exec, req, now); err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := attachGoalJudgeAttempt(ctx, exec, req, now); err != nil {
		return goal.JudgeAttempt{}, err
	}
	return getGoalJudgeAttemptWithExecutor(ctx, exec, req.Key, req.AttemptID)
}

func insertGoalJudgeAttempt(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.BeginJudgeAttemptRequest,
	now time.Time,
) error {
	err := sqlcgen.New(exec).InsertGoalJudgeAttempt(ctx, sqlcgen.InsertGoalJudgeAttemptParams{
		AttemptID: strings.TrimSpace(req.AttemptID), LoopRunID: string(req.Key.LoopRunID),
		Generation: int64(req.Key.Generation), NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		Turn: int64(req.Turn), JudgeDigest: strings.TrimSpace(req.JudgeDigest),
		UsageBaseTokens: req.UsageBaseTokens, StartedAt: store.FormatTimestamp(now),
	})
	if err != nil {
		return fmt.Errorf("store: insert goal judge attempt: %w", err)
	}
	return nil
}

func attachGoalJudgeAttempt(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.BeginJudgeAttemptRequest,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).AttachGoalJudgeAttempt(ctx, sqlcgen.AttachGoalJudgeAttemptParams{
		JudgeAttemptID: goalNullableString(req.AttemptID), UpdatedAt: store.FormatTimestamp(now),
		LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
		TaskRunID: goalNullableString(req.TaskRunID), PromptID: goalNullableString(req.PromptID),
	})
	if err != nil {
		return fmt.Errorf("store: attach goal judge attempt to checkpoint: %w", err)
	}
	return requireGoalAffectedCount(affected, "attach goal judge attempt")
}

// CompleteJudgeAttempt records the evaluator terminal exactly once.
func (g *GoalRepo) CompleteJudgeAttempt(
	ctx context.Context,
	req goal.CompleteJudgeAttemptRequest,
) (goal.JudgeAttempt, error) {
	if err := g.checkReady(ctx, "complete goal judge attempt"); err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := validateCompleteJudgeAttemptRequest(req); err != nil {
		return goal.JudgeAttempt{}, err
	}
	now := g.now()
	var completed goal.JudgeAttempt
	err := g.withTaskImmediateTransaction(ctx, "complete goal judge attempt", func(exec taskSQLExecutor) error {
		var err error
		completed, err = completeJudgeAttemptWithExecutor(ctx, exec, req, now)
		return err
	})
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	return completed, nil
}

func completeJudgeAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.CompleteJudgeAttemptRequest,
	now time.Time,
) (goal.JudgeAttempt, error) {
	if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
		return goal.JudgeAttempt{}, err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := validateJudgeCheckpointOwner(
		checkpoint,
		req.ExpectedControlEpoch,
		req.ExpectedBindingEpoch,
		req.TaskRunID,
		req.PromptID,
	); err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		req.Key,
		checkpoint.BindingHandle,
		req.ExpectedBindingEpoch,
		checkpoint.SessionID,
	); err != nil {
		return goal.JudgeAttempt{}, err
	}
	attempt, err := getGoalJudgeAttemptWithExecutor(ctx, exec, req.Key, req.AttemptID)
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	blockingJSON, err := json.Marshal(req.Verdict.BlockingIssues)
	if err != nil {
		return goal.JudgeAttempt{}, fmt.Errorf("store: encode goal judge blocking issues: %w", err)
	}
	evidenceRef, err := persistGoalVerdictEvidence(ctx, exec, req.Verdict, now)
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	if attempt.Status == goalJudgeStatusCompleted {
		if judgeAttemptMatchesCompletion(attempt, req, evidenceRef) {
			return attempt, nil
		}
		return goal.JudgeAttempt{}, goalControlStaleError(
			"judge attempt already completed with different terminal",
		)
	}
	if attempt.Status != goalJudgeStatusRunning {
		return goal.JudgeAttempt{}, goalControlStaleError("judge attempt is not running")
	}
	tokensUsed, err := goalSQLNullInt64(goalReportedTokens(req.TokensUsed, req.TokensReported))
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	affected, err := sqlcgen.New(exec).CompleteGoalJudgeAttempt(ctx, sqlcgen.CompleteGoalJudgeAttemptParams{
		Outcome: goalNullableString(string(req.Verdict.Outcome)), BlockingJson: string(blockingJSON),
		EvidenceRef: goalNullableString(evidenceRef), TokensUsed: tokensUsed, CompletedAt: store.FormatTimestamp(now),
		AttemptID: strings.TrimSpace(req.AttemptID), LoopRunID: string(req.Key.LoopRunID),
	})
	if err != nil {
		return goal.JudgeAttempt{}, fmt.Errorf("store: complete goal judge attempt: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "complete goal judge attempt"); err != nil {
		return goal.JudgeAttempt{}, err
	}
	return getGoalJudgeAttemptWithExecutor(ctx, exec, req.Key, req.AttemptID)
}

// GetJudgeAttempt returns one workspace-owned judge attempt.
func (g *GoalRepo) GetJudgeAttempt(
	ctx context.Context,
	key goal.TurnKey,
	attemptID string,
) (goal.JudgeAttempt, error) {
	if err := g.checkReady(ctx, "get goal judge attempt"); err != nil {
		return goal.JudgeAttempt{}, err
	}
	if err := validateGoalRunWorkspace(ctx, g.db, key); err != nil {
		return goal.JudgeAttempt{}, err
	}
	return getGoalJudgeAttemptWithExecutor(ctx, g.db, key, attemptID)
}

func getGoalJudgeAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	attemptID string,
) (goal.JudgeAttempt, error) {
	row, err := sqlcgen.New(exec).GetGoalJudgeAttempt(ctx, sqlcgen.GetGoalJudgeAttemptParams{
		AttemptID: strings.TrimSpace(attemptID), LoopRunID: string(key.LoopRunID),
		Generation: int64(key.Generation), NodeID: string(key.NodeID), ItemIndex: int64(key.ItemIndex),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goal.JudgeAttempt{}, fmt.Errorf("%w: judge attempt %s", goal.ErrTurnNotFound, attemptID)
	}
	if err != nil {
		return goal.JudgeAttempt{}, fmt.Errorf("store: scan goal judge attempt: %w", err)
	}
	attempt, err := goalJudgeAttemptFromGenerated(row, key.WorkspaceID)
	if err != nil {
		return goal.JudgeAttempt{}, err
	}
	return attempt, nil
}

func findGoalJudgeAttemptWithDigest(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	attemptID string,
) (goal.JudgeAttempt, bool, string, error) {
	digest, err := sqlcgen.New(exec).GetGoalJudgeAttemptDigest(ctx, sqlcgen.GetGoalJudgeAttemptDigestParams{
		AttemptID: strings.TrimSpace(attemptID), LoopRunID: string(key.LoopRunID),
		Generation: int64(key.Generation), NodeID: string(key.NodeID), ItemIndex: int64(key.ItemIndex),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goal.JudgeAttempt{}, false, "", nil
	}
	if err != nil {
		return goal.JudgeAttempt{}, false, "", err
	}
	attempt, err := getGoalJudgeAttemptWithExecutor(ctx, exec, key, attemptID)
	if err != nil {
		return goal.JudgeAttempt{}, false, "", err
	}
	return attempt, true, digest, nil
}

func goalReportedTokens(tokens int64, reported bool) any {
	if !reported {
		return nil
	}
	return tokens
}
