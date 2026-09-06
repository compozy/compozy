package store

import (
	"context"
	"fmt"
	"time"
)

// SessionInputClearError identifies the entry whose failed mutation rolled back a clear.
type SessionInputClearError struct {
	EntryID string
	Cause   error
}

func (e *SessionInputClearError) Error() string {
	return fmt.Sprintf("store: clear input %q: %v", e.EntryID, e.Cause)
}
func (e *SessionInputClearError) Unwrap() error { return e.Cause }

type SessionInputClearRequest struct {
	SessionID string
	TurnID    string
	ActorKind string
	ActorID   string
	Now       time.Time
}

type SessionInputClearResult struct {
	Inputs     []SessionInputQueueEntry
	Cleared    int
	Generation int64
}

type SessionInputClearTrace struct {
	EntryID    string
	SessionID  string
	TurnID     string
	ActorKind  string
	ActorID    string
	Generation int64
	CreatedAt  time.Time
}

type SessionInputClearStore interface {
	ClearSessionInputs(context.Context, SessionInputClearRequest) (SessionInputClearResult, error)
	ListPendingSessionInputClearTraces(context.Context, string) ([]SessionInputClearTrace, error)
	MarkSessionInputClearTraceProjected(context.Context, string, string, time.Time) error
}
