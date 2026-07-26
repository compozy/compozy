package goal

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
)

// BudgetDecision is the short-lived durable authorization presented at effect start.
type BudgetDecision struct {
	Allowed       bool
	Disposition   loop.ActionDisposition
	Cause         loop.ReasonCode
	BudgetVersion int64
	ValidUntil    time.Time
	GrantID       int64
	GrantTurn     int
	GrantScope    ControlGrantScope
}

// PreparePromptRequest inserts one non-dispatchable Goal-owned queue ticket idempotently.
type PreparePromptRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	PromptKind           string
	PromptAttempt        int
	SessionID            string
	BindingHandle        string
	BindingEpoch         int64
	UsageBaseTokens      int64
	UsageBaseReported    bool
	ContextUsageSequence *int64
	ContextUsageUsed     *int64
	Message              string
	PreparedAt           time.Time
}

// PromptTicket identifies one prepared managed input.
type PromptTicket struct {
	QueueEntryID string
	PromptID     string
}

// ClaimPreparedWorkPromptRequest atomically starts one durable work turn.
type ClaimPreparedWorkPromptRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	SessionID            string
	BindingHandle        string
	BindingEpoch         int64
	UsageBaseTokens      int64
	UsageBaseReported    bool
	BudgetDecision       BudgetDecision
	ActorKind            string
	ActorID              string
}

// ClaimPreparedCompactionRequest atomically starts compaction without allocating a turn.
type ClaimPreparedCompactionRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	SessionID            string
	BindingHandle        string
	BindingEpoch         int64
	UsageBaseTokens      int64
	UsageBaseReported    bool
	UsageSequence        *int64
	BudgetDecision       BudgetDecision
}

// ClaimedPrompt returns the raw one-time token while retaining only its digest on disk.
type ClaimedPrompt struct {
	Checkpoint    Checkpoint
	DispatchToken string
}

// DriverAttachmentRequest proves the live driver attached to the exact started operation.
type DriverAttachmentRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	SessionID            string
	BindingHandle        string
	DispatchToken        string
	DriverTurnID         string
	AttachedAt           time.Time
}

// FinalizePromptRequest writes exact queue terminal evidence under dispatch-token CAS.
type FinalizePromptRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	SessionID            string
	BindingHandle        string
	DispatchToken        string
	Result               loop.ActionPromptResult
	TerminalAt           time.Time
}

// RecoverPromptTerminalRequest commits terminal proof reconstructed from the exact session event range.
type RecoverPromptTerminalRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	SessionID            string
	BindingHandle        string
	Result               loop.ActionPromptResult
	TerminalAt           time.Time
}

// PromptTerminalRecoveryStore owns the boot-only no-raw-token terminal CAS.
type PromptTerminalRecoveryStore interface {
	RecoverGoalPromptTerminal(context.Context, RecoverPromptTerminalRequest) error
}

// FencePreparedPromptRequest fences or terminalizes a prepared prompt without a work turn.
type FencePreparedPromptRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	Outcome              loop.ActionPromptOutcome
	Disposition          loop.ActionDisposition
	Cause                loop.ReasonCode
	BudgetDecision       BudgetDecision
}

// RejectPreparedPromptRequest terminalizes one effect-known-false attempt and advances its retry identity.
type RejectPreparedPromptRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	ReasonCode           loop.ReasonCode
}

// PreparedPromptRejectionStore owns safe pre-submission retry advancement.
type PreparedPromptRejectionStore interface {
	RejectPreparedGoalPrompt(context.Context, RejectPreparedPromptRequest) error
}

// CompactionOutcome is the operational outcome of one correlated compact prompt.
type CompactionOutcome string

const (
	CompactionSucceeded   CompactionOutcome = "succeeded"
	CompactionIneffective CompactionOutcome = "ineffective"
	CompactionRefused     CompactionOutcome = "refused"
	CompactionCancelled   CompactionOutcome = "cancelled" //nolint:misspell // Persistence vocabulary is pinned.
	CompactionTimedOut    CompactionOutcome = "timed-out"
	CompactionFailed      CompactionOutcome = "failed"
)

// CompactionResult carries the exact managed prompt terminal and freshness evidence.
type CompactionResult struct {
	PromptResult      loop.ActionPromptResult
	Outcome           CompactionOutcome
	UsageSequence     *int64
	UsageBaselineUsed *int64
}

// CompleteCompactionRequest settles one started compact operation.
type CompleteCompactionRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	Result               CompactionResult
}

// CompleteTurnRequest atomically finalizes the existing open turn and projection.
type CompleteTurnRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	Result               loop.ActionPromptResult
	Verdict              *gate.Verdict
	DispatchActorKind    string
	DispatchActorID      string
}

// AmbiguousRequest finalizes an already-started effect without allocating a successor.
type AmbiguousRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	Cause                loop.ReasonCode
}

// RevokePromptRequest atomically fences prepared work or terminalizes one claimed operation.
type RevokePromptRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	Disposition          loop.ActionDisposition
	Status               string
	Cause                loop.ReasonCode
	ActorKind            string
	ActorID              string
	ProjectionCause      SessionOutboxCause
	RevokedAt            time.Time
}

// TurnStore is the complete Goal prompt/checkpoint persistence contract.
type TurnStore interface {
	CheckpointStore
	PrepareGoalPrompt(context.Context, PreparePromptRequest) (PromptTicket, error)
	ClaimPreparedWorkPrompt(context.Context, ClaimPreparedWorkPromptRequest) (ClaimedPrompt, error)
	ClaimPreparedCompaction(context.Context, ClaimPreparedCompactionRequest) (ClaimedPrompt, error)
	RecordGoalDriverAttached(context.Context, DriverAttachmentRequest) error
	FinalizeGoalPrompt(context.Context, FinalizePromptRequest) error
	FencePreparedPrompt(context.Context, FencePreparedPromptRequest) error
	CompleteTurn(context.Context, CompleteTurnRequest) (Checkpoint, error)
	CompleteCompaction(context.Context, CompleteCompactionRequest) (Checkpoint, error)
	MarkAmbiguous(context.Context, AmbiguousRequest) error
	RevokeGoalPrompt(context.Context, RevokePromptRequest) (Checkpoint, error)
}
