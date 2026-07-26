package globaldb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/loop/goal"
)

func validateBeginJudgeAttemptRequest(req goal.BeginJudgeAttemptRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(req.AttemptID) == "" || req.ExpectedControlEpoch < 1 ||
		req.ExpectedBindingEpoch < 1 || strings.TrimSpace(req.TaskRunID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || req.Turn < 1 ||
		strings.TrimSpace(req.JudgeDigest) == "" || req.UsageBaseTokens < 0 {
		return fmt.Errorf("%w: goal judge attempt identity is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func validateCompleteJudgeAttemptRequest(req goal.CompleteJudgeAttemptRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(req.AttemptID) == "" || req.ExpectedControlEpoch < 1 ||
		req.ExpectedBindingEpoch < 1 || strings.TrimSpace(req.TaskRunID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || !goalJudgeOutcomeValid(req.Verdict.Outcome) ||
		(req.TokensReported && req.TokensUsed < 0) {
		return fmt.Errorf("%w: goal judge completion is invalid", looppkg.ErrValidation)
	}
	return nil
}

func goalJudgeOutcomeValid(outcome gate.VerdictOutcome) bool {
	switch outcome {
	case gate.VerdictOutcomeApproved,
		gate.VerdictOutcomeRejected,
		gate.VerdictOutcomeBlocked,
		gate.VerdictOutcomeError,
		gate.VerdictOutcomeTimeout,
		gate.VerdictOutcomeInvalidOutput:
		return true
	default:
		return false
	}
}

func validateJudgeCheckpointOwner(
	checkpoint goal.Checkpoint,
	controlEpoch int64,
	bindingEpoch int64,
	taskRunID string,
	promptID string,
) error {
	if checkpoint.ControlEpoch != controlEpoch || checkpoint.BindingEpoch != bindingEpoch ||
		checkpoint.Phase != goalCheckpointPhaseJudging || checkpoint.TaskRunID != strings.TrimSpace(taskRunID) ||
		checkpoint.PromptID != strings.TrimSpace(promptID) {
		return goalControlStaleError("judge checkpoint owner changed")
	}
	return nil
}

func persistGoalVerdictEvidence(
	ctx context.Context,
	exec taskSQLExecutor,
	verdict gate.Verdict,
	now time.Time,
) (string, error) {
	if len(verdict.Payload) == 0 {
		return "", nil
	}
	if !json.Valid(verdict.Payload) {
		return "", fmt.Errorf("%w: goal judge evidence is invalid JSON", looppkg.ErrValidation)
	}
	ref := looppkg.OutputRefForPayload(verdict.Payload)
	if err := upsertLoopOutputBlobWithExecutor(ctx, exec, ref, verdict.Payload, now); err != nil {
		return "", err
	}
	return ref, nil
}

func judgeAttemptMatchesCompletion(
	attempt goal.JudgeAttempt,
	req goal.CompleteJudgeAttemptRequest,
	evidenceRef string,
) bool {
	if attempt.Outcome != string(req.Verdict.Outcome) || attempt.EvidenceRef != evidenceRef ||
		attempt.TokensReported != req.TokensReported {
		return false
	}
	if req.TokensReported && attempt.TokensUsed != req.TokensUsed {
		return false
	}
	wantBlocking, err := json.Marshal(req.Verdict.BlockingIssues)
	if err != nil {
		return false
	}
	gotBlocking, err := json.Marshal(attempt.BlockingIssues)
	return err == nil && bytes.Equal(gotBlocking, wantBlocking)
}
