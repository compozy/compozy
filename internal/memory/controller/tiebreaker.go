package controller

import (
	"context"
	"errors"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

var (
	// ErrTiebreakerDisabled tells the rule-first controller to keep its
	// deterministic ambiguity fallback without recording a failed model call.
	ErrTiebreakerDisabled = errors.New("memory controller: tiebreaker is disabled")
)

// TiebreakerRequest contains the bounded ambiguity context supplied to a model.
type TiebreakerRequest struct {
	Candidate memcontract.Candidate
	Targets   []Target
	RuleTrace []memcontract.RuleHit
}

// TiebreakerResult is the validated semantic result of one model call.
type TiebreakerResult struct {
	Op         memcontract.Op
	TargetID   string
	Confidence float32
	Reason     string
	Call       *memcontract.LLMCall
}

// Tiebreaker resolves only genuine ambiguity left by deterministic rules.
type Tiebreaker interface {
	BreakTie(context.Context, TiebreakerRequest) (TiebreakerResult, error)
}
