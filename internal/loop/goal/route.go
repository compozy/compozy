package goal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
)

const unexpectedAwaitingApprovalIssueID = "goal_judge_outcome_invalid"

func (e *Executor) judgeWorkTurn(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = checkpoint
	if checkpoint.JudgeAttemptID != "" {
		return e.recoverJudgeAttempt(ctx, segment)
	}
	turn := checkpoint.TurnsUsed
	if turn < 1 {
		return nil, fmt.Errorf("%w: Goal judge has no durably started turn", loop.ErrValidation)
	}
	attemptID, judgeDigest, err := deterministicJudgeIdentity(segment.key, turn, segment.params.Judge)
	if err != nil {
		return nil, err
	}
	operationBase := segment.usage.operationBase()
	decision, err := e.flushBudget(
		ctx,
		segment,
		BudgetBeforeJudge,
		checkpointPhaseJudging,
		turn,
		attemptID,
		operationBase,
	)
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		control, controlErr := e.budgetBoundary(ctx, segment, decision)
		return &turnBoundary{checkpoint: segment.checkpoint, result: result, control: control}, controlErr
	}
	attempt, err := e.store.BeginJudgeAttempt(ctx, BeginJudgeAttemptRequest{
		AttemptID:            attemptID,
		Key:                  segment.key,
		ExpectedControlEpoch: checkpoint.ControlEpoch,
		ExpectedBindingEpoch: checkpoint.BindingEpoch,
		TaskRunID:            segment.input.CorrelationID,
		PromptID:             result.PromptID,
		Turn:                 turn,
		JudgeDigest:          judgeDigest,
		UsageBaseTokens:      operationBase,
		BudgetDecision:       decision,
	})
	if err != nil {
		var reasonErr *loop.ReasonError
		if errors.As(err, &reasonErr) && reasonErr.Code == loop.ReasonCodeGoalPromptFenced {
			boundary, paused, pauseErr := e.settlePromptForPendingPause(ctx, segment, result)
			if paused || pauseErr != nil {
				return boundary, pauseErr
			}
		}
		return nil, err
	}
	return e.executeJudgeAttempt(ctx, segment, result, attempt, operationBase)
}

func (e *Executor) executeJudgeAttempt(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	attempt JudgeAttempt,
	operationBase int64,
) (*turnBoundary, error) {
	if attempt.Status == judgeAttemptStatusCompleted {
		segment.usage.observeOperation(attempt.UsageBaseTokens, attempt.TokensUsed, attempt.TokensReported)
		postDecision, flushErr := e.flushBudget(
			ctx,
			segment,
			BudgetAfterJudge,
			checkpointPhaseJudging,
			attempt.Turn,
			attempt.AttemptID,
			attempt.UsageBaseTokens,
		)
		if flushErr != nil {
			return nil, flushErr
		}
		return e.settleJudgeAttempt(ctx, segment, result, attempt, postDecision)
	}
	if attempt.Status != judgeAttemptStatusRunning {
		return nil, fmt.Errorf("%w: Goal judge attempt status %q cannot execute", loop.ErrValidation, attempt.Status)
	}
	judgeResult, evaluateErr := e.judge.EvaluateGoal(ctx, JudgeRequest{
		AttemptID: attempt.AttemptID,
		Key:       segment.key,
		Turn:      attempt.Turn,
		Criteria:  append([]dsl.GateCriterion(nil), segment.params.Judge...),
		Result:    result,
	})
	if evaluateErr != nil {
		judgeResult = brokenJudgeResult(evaluateErr)
	}
	judgeResult = normalizeUnexpectedJudgeOutcome(judgeResult)
	segment.usage.observeOperation(operationBase, judgeResult.TokensUsed, judgeResult.TokensReported)
	completed, err := e.store.CompleteJudgeAttempt(ctx, CompleteJudgeAttemptRequest{
		AttemptID:            attempt.AttemptID,
		Key:                  segment.key,
		ExpectedControlEpoch: segment.checkpoint.ControlEpoch,
		ExpectedBindingEpoch: segment.checkpoint.BindingEpoch,
		TaskRunID:            segment.input.CorrelationID,
		PromptID:             result.PromptID,
		Verdict:              judgeResult.Verdict,
		TokensUsed:           judgeResult.TokensUsed,
		TokensReported:       judgeResult.TokensReported,
	})
	if err != nil {
		return nil, err
	}
	postDecision, err := e.flushBudget(
		ctx,
		segment,
		BudgetAfterJudge,
		checkpointPhaseJudging,
		attempt.Turn,
		attempt.AttemptID,
		operationBase,
	)
	if err != nil {
		return nil, err
	}
	return e.settleJudgeAttempt(ctx, segment, result, completed, postDecision)
}

func (e *Executor) recoverJudgeAttempt(ctx context.Context, segment *segmentState) (*turnBoundary, error) {
	attempt, err := e.store.GetJudgeAttempt(ctx, segment.key, segment.checkpoint.JudgeAttemptID)
	if err != nil {
		return nil, err
	}
	switch attempt.Status {
	case judgeAttemptStatusRunning:
		err := e.store.MarkJudgeAmbiguous(ctx, MarkJudgeAmbiguousRequest{
			AttemptID:            attempt.AttemptID,
			Key:                  segment.key,
			ExpectedControlEpoch: segment.checkpoint.ControlEpoch,
			ExpectedBindingEpoch: segment.checkpoint.BindingEpoch,
			TaskRunID:            segment.input.CorrelationID,
			PromptID:             segment.checkpoint.PromptID,
			Cause:                string(loop.ReasonCodeGoalRecoveryAmbiguous),
		})
		if err != nil {
			return nil, err
		}
		updated, err := e.store.LoadCheckpoint(ctx, segment.key)
		if err != nil {
			return nil, err
		}
		segment.checkpoint = updated
		control, err := e.recoveredControl(ctx, segment)
		return &turnBoundary{checkpoint: updated, result: segment.lastResult, control: control, completed: true}, err
	case judgeAttemptStatusAmbiguous:
		control, err := e.checkpointBoundary(
			ctx,
			segment,
			loop.ActionDispositionNeedsApproval,
			goalStatusPaused,
			loop.ReasonCodeGoalRecoveryAmbiguous,
			automaticActor(),
		)
		return &turnBoundary{checkpoint: segment.checkpoint, control: control, completed: true}, err
	case judgeAttemptStatusCompleted:
		segment.usage.observeOperation(attempt.UsageBaseTokens, attempt.TokensUsed, attempt.TokensReported)
		decision, err := e.flushBudget(
			ctx,
			segment,
			BudgetAfterJudge,
			checkpointPhaseJudging,
			attempt.Turn,
			attempt.AttemptID,
			attempt.UsageBaseTokens,
		)
		if err != nil {
			return nil, err
		}
		return e.settleJudgeAttempt(ctx, segment, segment.lastResult, attempt, decision)
	default:
		return nil, fmt.Errorf("%w: Goal judge attempt status %q is invalid", loop.ErrValidation, attempt.Status)
	}
}

func (e *Executor) settleJudgeAttempt(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	attempt JudgeAttempt,
	postDecision BudgetDecision,
) (*turnBoundary, error) {
	if boundary, handled, err := e.settleUnexpectedJudgeOutcome(ctx, segment, result, attempt); handled || err != nil {
		return boundary, err
	}
	verdict := verdictFromAttempt(attempt)
	previousBrokenStreak := segment.checkpoint.BrokenStreak
	settled, err := e.completeTurn(ctx, segment, result, &verdict)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = settled
	segment.lastVerdict = string(verdict.Outcome)
	segment.lastBlockingIssues = append([]gate.BlockingIssue(nil), verdict.BlockingIssues...)
	if boundary, handled, err := e.judgeOutcomeBoundary(
		ctx,
		segment,
		result,
		&verdict,
		previousBrokenStreak,
	); handled || err != nil {
		return boundary, err
	}
	if control, ok, pauseErr := e.pendingPauseBoundary(ctx, segment); ok || pauseErr != nil {
		boundary := &turnBoundary{
			checkpoint: settled,
			result:     result,
			verdict:    &verdict,
			control:    control,
			completed:  true,
		}
		return boundary, pauseErr
	}
	if !postDecision.Allowed {
		control, err := e.budgetBoundary(ctx, segment, postDecision)
		boundary := &turnBoundary{
			checkpoint: settled,
			result:     result,
			verdict:    &verdict,
			control:    control,
			completed:  true,
		}
		return boundary, err
	}
	if settled.TurnsUsed >= settled.TurnLimit {
		return e.exhaustedBoundary(ctx, segment)
	}
	return &turnBoundary{checkpoint: settled, result: result, verdict: &verdict, completed: true}, nil
}

func (e *Executor) settleUnexpectedJudgeOutcome(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	attempt JudgeAttempt,
) (*turnBoundary, bool, error) {
	if !judgeAttemptWasUnexpectedAwaitingApproval(attempt) {
		return nil, false, nil
	}
	settled, err := e.completeTurn(ctx, segment, result, nil)
	if err != nil {
		return nil, true, err
	}
	segment.checkpoint = settled
	if control, ok, pauseErr := e.pendingPauseBoundary(ctx, segment); ok || pauseErr != nil {
		boundary := &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}
		return boundary, true, pauseErr
	}
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionNeedsApproval,
		goalStatusPaused,
		loop.ReasonCodeGoalJudgeOutcomeInvalid,
		automaticActor(),
	)
	boundary := &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}
	return boundary, true, err
}

func (e *Executor) judgeOutcomeBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	verdict *gate.Verdict,
	previousBrokenStreak int,
) (*turnBoundary, bool, error) {
	switch verdict.Outcome {
	case gate.VerdictOutcomeApproved:
		return e.terminalVerdictBoundary(
			ctx, segment, result, verdict, loop.ActionDispositionSucceeded, goalStatusComplete, "",
		)
	case gate.VerdictOutcomeBlocked:
		return e.terminalVerdictBoundary(
			ctx,
			segment,
			result,
			verdict,
			loop.ActionDispositionBlocked,
			goalStatusBlocked,
			loop.ReasonCodeGoalReportedBlocked,
		)
	case gate.VerdictOutcomeAwaitingApproval:
		return e.terminalVerdictBoundary(
			ctx,
			segment,
			result,
			verdict,
			loop.ActionDispositionNeedsApproval,
			goalStatusPaused,
			loop.ReasonCodeGoalJudgeOutcomeInvalid,
		)
	case gate.VerdictOutcomeError, gate.VerdictOutcomeTimeout, gate.VerdictOutcomeInvalidOutput:
		return e.brokenVerdictBoundary(ctx, segment, result, verdict, previousBrokenStreak)
	case gate.VerdictOutcomeRejected:
		return nil, false, nil
	default:
		err := fmt.Errorf("%w: Goal judge outcome %q is invalid", loop.ErrValidation, verdict.Outcome)
		return nil, true, err
	}
}

func (e *Executor) terminalVerdictBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	verdict *gate.Verdict,
	disposition loop.ActionDisposition,
	status string,
	cause loop.ReasonCode,
) (*turnBoundary, bool, error) {
	control, err := e.checkpointBoundary(ctx, segment, disposition, status, cause, automaticActor())
	boundary := &turnBoundary{
		checkpoint: segment.checkpoint,
		result:     result,
		verdict:    verdict,
		control:    control,
		completed:  true,
	}
	return boundary, true, err
}

func (e *Executor) brokenVerdictBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	verdict *gate.Verdict,
	previousBrokenStreak int,
) (*turnBoundary, bool, error) {
	if segment.checkpoint.BrokenStreak == 0 {
		segment.checkpoint.BrokenStreak = previousBrokenStreak + 1
	}
	if segment.checkpoint.BrokenStreak < brokenJudgeStreakLimit {
		return nil, false, nil
	}
	return e.terminalVerdictBoundary(
		ctx,
		segment,
		result,
		verdict,
		loop.ActionDispositionNeedsApproval,
		goalStatusPaused,
		loop.ReasonCodeGoalJudgeBroken,
	)
}

func normalizeUnexpectedJudgeOutcome(result JudgeResult) JudgeResult {
	if result.Verdict.Outcome != gate.VerdictOutcomeAwaitingApproval {
		return result
	}
	result.Verdict.Outcome = gate.VerdictOutcomeInvalidOutput
	result.Verdict.Broken = true
	result.Verdict.BlockingIssues = append(
		append([]gate.BlockingIssue(nil), result.Verdict.BlockingIssues...),
		gate.BlockingIssue{
			ID:   unexpectedAwaitingApprovalIssueID,
			Note: "Goal judge returned the forbidden awaiting_approval outcome",
		},
	)
	return result
}

func judgeAttemptWasUnexpectedAwaitingApproval(attempt JudgeAttempt) bool {
	if attempt.Outcome != string(gate.VerdictOutcomeInvalidOutput) {
		return false
	}
	for _, issue := range attempt.BlockingIssues {
		if strings.TrimSpace(issue.ID) == unexpectedAwaitingApprovalIssueID {
			return true
		}
	}
	return false
}

func (e *Executor) hydratePriorJudgeContext(ctx context.Context, segment *segmentState) error {
	checkpoint := segment.checkpoint
	if checkpoint.Phase != checkpointPhaseIdle || checkpoint.TurnsUsed < 1 {
		return nil
	}
	attemptID, _, err := deterministicJudgeIdentity(
		segment.key,
		checkpoint.TurnsUsed,
		segment.params.Judge,
	)
	if err != nil {
		return err
	}
	attempt, err := e.store.GetJudgeAttempt(ctx, segment.key, attemptID)
	if errors.Is(err, ErrTurnNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	switch attempt.Status {
	case judgeAttemptStatusAmbiguous:
		return nil
	case judgeAttemptStatusCompleted:
		segment.lastVerdict = strings.TrimSpace(attempt.Outcome)
		segment.lastBlockingIssues = append([]gate.BlockingIssue(nil), attempt.BlockingIssues...)
		return nil
	default:
		return fmt.Errorf(
			"%w: idle Goal checkpoint references prior judge status %q",
			loop.ErrValidation,
			attempt.Status,
		)
	}
}

func verdictFromAttempt(attempt JudgeAttempt) gate.Verdict {
	return gate.Verdict{
		Outcome:        gate.VerdictOutcome(strings.TrimSpace(attempt.Outcome)),
		BlockingIssues: append([]gate.BlockingIssue(nil), attempt.BlockingIssues...),
	}
}

func brokenJudgeResult(err error) JudgeResult {
	return JudgeResult{Verdict: gate.Verdict{
		Outcome: gate.VerdictOutcomeError,
		Broken:  true,
		BlockingIssues: []gate.BlockingIssue{{
			ID:   "goal_judge_unavailable",
			Note: err.Error(),
		}},
	}}
}
