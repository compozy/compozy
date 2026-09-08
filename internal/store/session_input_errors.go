package store

import "fmt"

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
