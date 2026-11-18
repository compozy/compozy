package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrSubagentPinFollowsParent reports a pin request for a subagent session,
	// which lives in its parent's subagent roster rather than in the session list.
	ErrSubagentPinFollowsParent = errors.New("store: subagent sessions follow their parent and cannot be pinned")
	// ErrSessionPinInstanceMain reports a pin request for an always-on agent's main
	// session, which lives in its avatar rather than in the session list.
	ErrSessionPinInstanceMain = errors.New("store: an always-on agent's main session cannot be pinned")
)

// SessionPinnedFilter controls whether pinned rows participate in a catalog read.
type SessionPinnedFilter string

const (
	SessionPinnedExclude SessionPinnedFilter = "exclude"
	SessionPinnedOnly    SessionPinnedFilter = "only"
	SessionPinnedInclude SessionPinnedFilter = "include"
)

// Validate reports whether the pinned filter is supported. An empty filter
// leaves the catalog read unfiltered, like include.
func (f SessionPinnedFilter) Validate() error {
	switch f {
	case "", SessionPinnedExclude, SessionPinnedOnly, SessionPinnedInclude:
		return nil
	default:
		return fmt.Errorf("store: unsupported session pinned filter %q", f)
	}
}

// SessionPinStore owns the operator's pin on a session. A pin is organization
// only: setting or clearing it never touches the session's activity
// timestamps, so the session keeps its place in every other ordering.
type SessionPinStore interface {
	// PinSession pins one session; a pinned session keeps its original pin time.
	PinSession(ctx context.Context, workspaceID string, sessionID string) (SessionInfo, error)
	// UnpinSession clears the pin; it reports false when the session was not pinned.
	UnpinSession(ctx context.Context, workspaceID string, sessionID string) (SessionInfo, bool, error)
	// SessionPinnedAt reads the pin time of each listed session that is pinned.
	SessionPinnedAt(ctx context.Context, sessionIDs []string) (map[string]time.Time, error)
}
