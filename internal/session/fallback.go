package session

import "github.com/compozy/compozy/internal/acp"

// ChainOwner declares who runs the fallback chain for one launch, so no launch ever has
// two chain owners (ADR-004).
type ChainOwner uint8

const (
	// ChainOwnerSession is the default: the session layer owns the agent's fallback_chain
	// for ordinary work sessions (HTTP/CLI/tool Create, first binds, agent-requested Spawn).
	ChainOwnerSession ChainOwner = iota
	// ChainOwnerCaller means a caller (a background role) owns its chain; the session layer
	// performs exactly the requested route and reports the structured acceptance outcome.
	ChainOwnerCaller
)

// StartAccepted reports whether a start/bind attempt reached ACP acceptance: a nil error,
// or an error carrying acp.AcceptedStartError anywhere in its chain. It is the only
// acceptance derivation chain owners use; a nil or non-nil process never decides it.
func StartAccepted(err error) bool {
	if err == nil {
		return true
	}
	_, accepted := acp.AcceptedSessionID(err)
	return accepted
}
