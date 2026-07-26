package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
)

// Turn is one immutable-start, single-terminal-transition Goal work audit row.
type Turn struct {
	Key             TurnKey
	Seq             int64
	Turn            int
	SessionID       string
	BindingHandle   string
	BindingEpoch    int64
	PromptID        string
	PromptAttempt   int
	UsageBaseTokens int64
	ResultStatus    *string
	StopReason      *loop.ActionStopReason
	ReasonCode      *loop.ReasonCode
	VerdictOutcome  *gate.VerdictOutcome
	BlockingIssues  []gate.BlockingIssue
	EvidenceRef     *string
	PromptRef       *string
	TokensUsed      *int64
	ActorKind       string
	ActorID         string
	StartedAt       time.Time
	EndedAt         *time.Time
}

// TurnQuery filters run-wide sequence pagination without weakening workspace ownership.
type TurnQuery struct {
	WorkspaceID loop.WorkspaceID
	LoopRunID   loop.RunID
	NodeID      dsl.NodeID
	ItemIndex   *int
	AfterSeq    int64
	Limit       int
}

// Validate rejects ambiguous cursors and invalid filter ranges.
func (q TurnQuery) Validate() error {
	if err := (TurnKey{
		WorkspaceID: q.WorkspaceID,
		LoopRunID:   q.LoopRunID,
		Generation:  1,
		NodeID:      "query",
	}).Validate(); err != nil {
		return err
	}
	if q.AfterSeq < 0 {
		return fmt.Errorf("%w: goal after_seq must be nonnegative", loop.ErrValidation)
	}
	if q.Limit < 0 || q.Limit > 200 {
		return fmt.Errorf("%w: goal turn limit must be between 0 and 200", loop.ErrValidation)
	}
	if q.ItemIndex != nil && *q.ItemIndex < 0 {
		return fmt.Errorf("%w: goal item_index must be nonnegative", loop.ErrValidation)
	}
	if q.NodeID != "" && strings.TrimSpace(string(q.NodeID)) == "" {
		return fmt.Errorf("%w: goal node_id must be nonempty when present", loop.ErrValidation)
	}
	if q.ItemIndex != nil && strings.TrimSpace(string(q.NodeID)) == "" {
		return fmt.Errorf("%w: goal item_index requires node_id", loop.ErrValidation)
	}
	return nil
}

// TurnPage is the run-wide monotonic cursor page.
type TurnPage struct {
	Turns        []Turn
	NextAfterSeq *int64
}

// TurnReader loads immutable Goal audit history by run-wide sequence.
type TurnReader interface {
	ListGoalTurns(context.Context, TurnQuery) (TurnPage, error)
	GetGoalTurnByPromptID(context.Context, TurnKey, string) (Turn, error)
}
