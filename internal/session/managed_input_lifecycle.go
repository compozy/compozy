package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
)

// ManagedInputOwner is the complete identity persisted on a managed queue row.
type ManagedInputOwner struct {
	QueueEntryID  string
	SessionID     string
	OwnerKind     string
	LoopRunID     string
	TaskRunID     string
	RunGeneration int
	PromptAttempt int
	ControlEpoch  int64
	BindingEpoch  int64
	PromptID      string
	PromptKind    string
}

// ManagedInputClaim asks the daemon lifecycle to perform the durable start linearization.
type ManagedInputClaim struct {
	Owner       ManagedInputOwner
	RequestedAt time.Time
}

// ManagedInputPromptMeta is persisted with transcript events without importing Loop.
type ManagedInputPromptMeta struct {
	LoopRunID     string
	NodeID        string
	PromptID      string
	Kind          string
	Generation    int
	ItemIndex     int
	PromptAttempt int
	Turn          *int
}

// ManagedInputSubmission proves a claim/start commit and returns its one-time token.
type ManagedInputSubmission struct {
	Owner         ManagedInputOwner
	PromptMeta    ManagedInputPromptMeta
	DispatchToken string
	BudgetVersion int64
	StartedAt     time.Time
	UsageReporter ManagedInputUsageReporter
}

// ManagedInputUsageReporter receives cumulative usage for one managed operation.
type ManagedInputUsageReporter interface {
	ReportActionTokensUsed(int64)
}

// ManagedInputReceipt correlates post-claim driver evidence.
type ManagedInputReceipt struct {
	Owner         ManagedInputOwner
	DispatchToken string
	DriverTurnID  string
	ReasonCode    string
}

// ManagedInputRejection records a pre-claim effect-known-false rejection.
type ManagedInputRejection struct {
	Owner            ManagedInputOwner
	ReasonCode       string
	EffectKnownFalse bool
}

// ManagedInputTerminal is the exact persisted terminal evidence from the prompt pump.
type ManagedInputTerminal struct {
	Owner          ManagedInputOwner
	DispatchToken  string
	DriverTurnID   string
	Outcome        string
	Text           string
	Structured     []byte
	EventStartSeq  int64
	EventEndSeq    int64
	TokensUsed     int64
	TokensReported bool
	StopReason     acp.PromptStopReason
	ReasonCode     string
	TerminalAt     time.Time
}

// ManagedInputEffectError carries typed certainty through errors.As.
type ManagedInputEffectError struct {
	Owner      ManagedInputOwner
	Certainty  EffectCertainty
	ReasonCode string
	Finalized  bool
	Err        error
}

func (e *ManagedInputEffectError) Error() string {
	if e == nil {
		return "session: managed input effect failed"
	}
	if e.Err == nil {
		return fmt.Sprintf("session: managed input effect failed (%s, %s)", e.ReasonCode, e.Certainty)
	}
	return fmt.Sprintf(
		"session: managed input effect failed (%s, %s): %v",
		strings.TrimSpace(e.ReasonCode),
		e.Certainty,
		e.Err,
	)
}

func (e *ManagedInputEffectError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ManagedInputLifecycle is the neutral daemon callback used inside the real prompt slot.
type ManagedInputLifecycle interface {
	BeginSubmission(context.Context, ManagedInputClaim) (ManagedInputSubmission, error)
	RecordDriverAttached(context.Context, ManagedInputReceipt) error
	RecordRejected(context.Context, ManagedInputRejection) error
	RecordAmbiguous(context.Context, ManagedInputReceipt) error
	RecordTerminal(context.Context, ManagedInputTerminal) error
	Revoke(context.Context, ManagedInputOwner, string) error
}
