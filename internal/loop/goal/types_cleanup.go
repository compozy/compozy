package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
)

// SessionCleanupCause identifies the transaction that retired a run-owned session binding.
type SessionCleanupCause string

const (
	SessionCleanupCauseTerminal       SessionCleanupCause = "terminal"
	SessionCleanupCauseReseed         SessionCleanupCause = "reseed"
	SessionCleanupCauseControlRevoked SessionCleanupCause = "control-revoked"
	SessionCleanupCauseStop           SessionCleanupCause = "stop"
)

// SessionCleanupObligation is one durable, idempotently acknowledged session Stop effect.
type SessionCleanupObligation struct {
	ID           int64
	CleanupID    string
	WorkspaceID  loop.WorkspaceID
	LoopRunID    loop.RunID
	Handle       string
	BindingEpoch int64
	SessionID    string
	Cause        SessionCleanupCause
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

// Validate enforces the complete run-owned cleanup identity.
func (o SessionCleanupObligation) Validate() error {
	if strings.TrimSpace(o.CleanupID) == "" || strings.TrimSpace(string(o.WorkspaceID)) == "" ||
		strings.TrimSpace(string(o.LoopRunID)) == "" || strings.TrimSpace(o.Handle) == "" ||
		o.BindingEpoch < 1 || strings.TrimSpace(o.SessionID) == "" || !o.Cause.Valid() || o.CreatedAt.IsZero() {
		return fmt.Errorf("%w: Goal session cleanup identity is incomplete", loop.ErrValidation)
	}
	if o.CompletedAt != nil && o.CompletedAt.Before(o.CreatedAt) {
		return fmt.Errorf("%w: Goal session cleanup completion precedes creation", loop.ErrValidation)
	}
	return nil
}

// Valid reports whether the cleanup cause belongs to the closed persistence vocabulary.
func (c SessionCleanupCause) Valid() bool {
	switch c {
	case SessionCleanupCauseTerminal,
		SessionCleanupCauseReseed,
		SessionCleanupCauseControlRevoked,
		SessionCleanupCauseStop:
		return true
	default:
		return false
	}
}

// SessionCleanupStore owns pending cleanup delivery and acknowledgement.
type SessionCleanupStore interface {
	ClaimGoalSessionCleanup(context.Context, int) ([]SessionCleanupObligation, error)
	AcknowledgeGoalSessionCleanup(context.Context, string, time.Time) error
}
