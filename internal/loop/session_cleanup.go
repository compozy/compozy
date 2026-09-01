package loop

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SessionCleanupSourceKind identifies the durable owner of a Loop session.
type SessionCleanupSourceKind string

const (
	SessionCleanupSourceGoalBinding SessionCleanupSourceKind = "goal-binding"
	SessionCleanupSourceTaskRun     SessionCleanupSourceKind = "task-run"
)

// SessionCleanupCause identifies the transaction that retired a run-owned session.
type SessionCleanupCause string

const (
	SessionCleanupCauseTerminal       SessionCleanupCause = "terminal"
	SessionCleanupCauseReseed         SessionCleanupCause = "reseed"
	SessionCleanupCauseControlRevoked SessionCleanupCause = "control-revoked"
	SessionCleanupCauseStop           SessionCleanupCause = "stop"
	SessionCleanupCauseOperatorCancel SessionCleanupCause = "operator-cancel"
)

// SessionCleanupObligation is one durable, idempotently acknowledged session Stop effect.
type SessionCleanupObligation struct {
	ID          int64
	CleanupID   string
	WorkspaceID WorkspaceID
	LoopRunID   RunID
	SourceKind  SessionCleanupSourceKind
	SourceID    string
	SourceEpoch int64
	SessionID   string
	Cause       SessionCleanupCause
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// Validate enforces the complete run-owned cleanup identity.
func (o SessionCleanupObligation) Validate() error {
	if strings.TrimSpace(o.CleanupID) == "" || strings.TrimSpace(string(o.WorkspaceID)) == "" ||
		strings.TrimSpace(string(o.LoopRunID)) == "" || strings.TrimSpace(o.SourceID) == "" ||
		strings.TrimSpace(o.SessionID) == "" || !o.SourceKind.Valid() || !o.Cause.Valid() || o.CreatedAt.IsZero() {
		return fmt.Errorf("%w: Loop session cleanup identity is incomplete", ErrValidation)
	}
	switch o.SourceKind {
	case SessionCleanupSourceGoalBinding:
		if o.SourceEpoch < 1 {
			return fmt.Errorf("%w: Goal binding cleanup epoch must be positive", ErrValidation)
		}
	case SessionCleanupSourceTaskRun:
		if o.SourceEpoch != 0 {
			return fmt.Errorf("%w: task-run cleanup epoch must be zero", ErrValidation)
		}
	}
	if o.CompletedAt != nil && o.CompletedAt.Before(o.CreatedAt) {
		return fmt.Errorf("%w: Loop session cleanup completion precedes creation", ErrValidation)
	}
	return nil
}

// Valid reports whether source belongs to the closed persistence vocabulary.
func (k SessionCleanupSourceKind) Valid() bool {
	switch k {
	case SessionCleanupSourceGoalBinding, SessionCleanupSourceTaskRun:
		return true
	default:
		return false
	}
}

// Valid reports whether cause belongs to the closed persistence vocabulary.
func (c SessionCleanupCause) Valid() bool {
	switch c {
	case SessionCleanupCauseTerminal,
		SessionCleanupCauseReseed,
		SessionCleanupCauseControlRevoked,
		SessionCleanupCauseStop,
		SessionCleanupCauseOperatorCancel:
		return true
	default:
		return false
	}
}

// SessionCleanupStore owns pending cleanup delivery and acknowledgement.
type SessionCleanupStore interface {
	ClaimLoopSessionCleanup(context.Context, int) ([]SessionCleanupObligation, error)
	AcknowledgeLoopSessionCleanup(context.Context, string, time.Time) error
}
