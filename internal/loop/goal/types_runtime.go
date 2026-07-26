package goal

import (
	"context"

	"github.com/compozy/agh/internal/loop"
)

// PromptRecoveryIdentity is the complete persisted identity eligible for event reconciliation.
type PromptRecoveryIdentity struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	QueueEntryID         string
	PromptID             string
	SessionID            string
}

// PromptRecovery proves an exact terminal result from correlated session events.
type PromptRecovery interface {
	ReconcileTerminalFromEvents(
		context.Context,
		PromptRecoveryIdentity,
	) (loop.ActionPromptResult, bool, error)
}

// BudgetBoundary identifies one synchronous nested-budget fence.
type BudgetBoundary string

const (
	BudgetBeforeWork    BudgetBoundary = "before_work"
	BudgetAfterWork     BudgetBoundary = "after_work"
	BudgetBeforeCompact BudgetBoundary = "before_compact"
	BudgetAfterCompact  BudgetBoundary = "after_compact"
	BudgetBeforeJudge   BudgetBoundary = "before_judge"
	BudgetAfterJudge    BudgetBoundary = "after_judge"
)

// BudgetBoundarySnapshot carries cumulative task usage at one effect boundary.
type BudgetBoundarySnapshot struct {
	Key                  TurnKey
	TaskRunID            string
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	ExpectedPhase        string
	ExpectedQueueEntryID string
	ExpectedPromptID     string
	Boundary             BudgetBoundary
	Phase                string
	Turn                 int64
	OperationID          string
	OperationBaseTokens  int64
	LiveTokensUsed       int64
	TokensReported       bool
}

// BudgetGuard synchronously flushes usage and returns a short-lived durable decision.
type BudgetGuard interface {
	FlushAndCheck(context.Context, BudgetBoundarySnapshot) (BudgetDecision, error)
}

// ExecutorStore is the durable Goal state needed by the convergence worker.
type ExecutorStore interface {
	TurnStore
	JudgeStore
}
