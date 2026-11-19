package store

import (
	"context"
	"errors"
	"fmt"
)

var ErrSessionInputSelectionChanged = errors.New("store: selected session input is no longer the queue head")

type promptControlOnlyKey struct{}

// WithPromptControlOnly records that the parsed command cannot invoke an ACP provider.
func WithPromptControlOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, promptControlOnlyKey{}, true)
}

func PromptControlOnly(ctx context.Context) bool {
	control, _ := ctx.Value(promptControlOnlyKey{}).(bool)
	return control
}

type SessionInputQueueFullError struct {
	SessionID string
	Cap       int
	Count     int
}

func (e *SessionInputQueueFullError) Error() string {
	return fmt.Sprintf("%v: session %s cap %d count %d", ErrSessionInputQueueFull, e.SessionID, e.Cap, e.Count)
}

func (e *SessionInputQueueFullError) Unwrap() error { return ErrSessionInputQueueFull }

type SessionInputNotQueuedError struct {
	EntryID string
	Status  string
	Text    string
}

func (e *SessionInputNotQueuedError) Error() string {
	return fmt.Sprintf("%v: %s is %s", ErrSessionInputQueueEntryNotQueued, e.EntryID, e.Status)
}

func (e *SessionInputNotQueuedError) Unwrap() error { return ErrSessionInputQueueEntryNotQueued }
