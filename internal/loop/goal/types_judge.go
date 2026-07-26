package goal

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
)

// JudgeAttempt is the durable persist-before-effect aggregate judge operation.
type JudgeAttempt struct {
	AttemptID       string
	Key             TurnKey
	Turn            int
	Status          string
	Outcome         string
	BlockingIssues  []gate.BlockingIssue
	EvidenceRef     string
	TokensUsed      int64
	TokensReported  bool
	UsageBaseTokens int64
	StartedAt       time.Time
	CompletedAt     *time.Time
}

// BeginJudgeAttemptRequest records evaluator intent under Goal CAS.
type BeginJudgeAttemptRequest struct {
	AttemptID            string
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	PromptID             string
	Turn                 int
	JudgeDigest          string
	UsageBaseTokens      int64
	BudgetDecision       BudgetDecision
}

// CompleteJudgeAttemptRequest records one typed evaluator terminal.
type CompleteJudgeAttemptRequest struct {
	AttemptID            string
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	PromptID             string
	Verdict              gate.Verdict
	TokensUsed           int64
	TokensReported       bool
}

// MarkJudgeAmbiguousRequest terminalizes a running judge without re-execution.
type MarkJudgeAmbiguousRequest struct {
	AttemptID            string
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	PromptID             string
	Cause                string
}

// JudgeRequest carries the pinned criteria for one evaluator operation.
type JudgeRequest struct {
	AttemptID string
	Key       TurnKey
	Turn      int
	Criteria  []dsl.GateCriterion
	Result    loop.ActionPromptResult
}

// JudgeResult carries the evaluator verdict and nullable-token truth.
type JudgeResult struct {
	Verdict        gate.Verdict
	TokensUsed     int64
	TokensReported bool
}

// JudgeEvaluator executes one complete Goal criteria bundle.
type JudgeEvaluator interface {
	EvaluateGoal(context.Context, JudgeRequest) (JudgeResult, error)
}

// JudgeStore persists aggregate judge attempts.
type JudgeStore interface {
	BeginJudgeAttempt(context.Context, BeginJudgeAttemptRequest) (JudgeAttempt, error)
	CompleteJudgeAttempt(context.Context, CompleteJudgeAttemptRequest) (JudgeAttempt, error)
	MarkJudgeAmbiguous(context.Context, MarkJudgeAmbiguousRequest) error
	GetJudgeAttempt(context.Context, TurnKey, string) (JudgeAttempt, error)
}
