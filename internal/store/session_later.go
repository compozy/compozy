package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrSessionInLater reports an attempt to reactivate a session parked in Later.
	ErrSessionInLater = errors.New("store: session is in later")
	// ErrSessionLaterRequiresStopped reports a park attempt for a live session.
	ErrSessionLaterRequiresStopped = errors.New("store: session must be stopped before moving to later")
)

// SessionLater is the persisted Later state of a stopped session. ReturnAt is
// the optional wall-clock time when the daemon brings the session back.
type SessionLater struct {
	ParkedAt time.Time
	Reason   string
	ReturnAt *time.Time
	// PendingMessages counts agent messages queued while the session is parked.
	PendingMessages int
}

// CloneSessionLater returns an isolated copy of optional Later state.
func CloneSessionLater(later *SessionLater) *SessionLater {
	if later == nil {
		return nil
	}
	cloned := *later
	if later.ReturnAt != nil {
		cloned.ReturnAt = new(*later.ReturnAt)
	}
	return &cloned
}

// SessionLaterFilter controls whether parked rows participate in a catalog read.
type SessionLaterFilter string

const (
	SessionLaterExclude SessionLaterFilter = "exclude"
	SessionLaterOnly    SessionLaterFilter = "only"
	SessionLaterInclude SessionLaterFilter = "include"
)

// Validate reports whether the Later filter is supported. An empty filter
// leaves internal catalog reads unfiltered.
func (f SessionLaterFilter) Validate() error {
	switch f {
	case "", SessionLaterExclude, SessionLaterOnly, SessionLaterInclude:
		return nil
	default:
		return fmt.Errorf("store: unsupported session later filter %q", f)
	}
}

// SessionLaterPark parks one stopped session. Reason and ReturnAt belong to the
// session the operator parked; descendants parked with it carry neither.
type SessionLaterPark struct {
	WorkspaceID string
	SessionID   string
	Reason      string
	ReturnAt    *time.Time
}

// SessionLaterReturn names a parked session the daemon brought back because its
// return time passed.
type SessionLaterReturn struct {
	WorkspaceID string
	SessionID   string
	ProfileID   string
	Name        string
	Reason      string
	ReturnedAt  time.Time
}

// SessionLaterStore owns the persisted Later state of sessions.
type SessionLaterStore interface {
	// SessionLaterState reads the Later state and back-from-Later mark of one session.
	SessionLaterState(ctx context.Context, workspaceID string, sessionID string) (*SessionLater, *time.Time, error)
	// ParkSession marks one stopped, unarchived session as parked.
	ParkSession(ctx context.Context, park SessionLaterPark) (SessionInfo, error)
	// UnparkSession clears Later state; it reports false when the session was not parked.
	UnparkSession(ctx context.Context, workspaceID string, sessionID string) (bool, error)
	// UnparkSessions clears Later state for every listed session in one transaction.
	UnparkSessions(ctx context.Context, workspaceID string, sessionIDs []string) error
	// UnparkSessionParkedAt clears Later state only when it is still the park made at parkedAt.
	UnparkSessionParkedAt(ctx context.Context, workspaceID string, sessionID string, parkedAt time.Time) (bool, error)
	// ReturnDueLaterSessions brings back at most limit parked sessions whose
	// return time is at or before now, marking each as back from Later.
	ReturnDueLaterSessions(ctx context.Context, now time.Time, limit int) ([]SessionLaterReturn, error)
	// ClearLaterReturn clears the back-from-Later mark; it reports whether one was set.
	ClearLaterReturn(ctx context.Context, sessionID string) (bool, error)
}

// LaterState returns the parked state of the session; nil when it is not parked.
func (s *SessionInfo) LaterState() *SessionLater {
	if s == nil || s.Attention == nil {
		return nil
	}
	return s.Attention.Later
}

// LaterReturnedAtValue returns when the daemon brought the session back from Later.
func (s *SessionInfo) LaterReturnedAtValue() *time.Time {
	if s == nil || s.Attention == nil {
		return nil
	}
	return s.Attention.LaterReturnedAt
}
