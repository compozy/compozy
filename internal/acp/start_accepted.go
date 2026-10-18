package acp

import (
	"errors"
	"strings"
)

// AcceptedStartError reports a failure that happened after ACP accepted the session:
// session/new or session/load returned a session id. Fallback-chain owners must stop on
// it; the accepted route is final for that binding even though the start failed.
type AcceptedStartError struct {
	SessionID string
	Cause     error
}

// Error implements error.
func (e *AcceptedStartError) Error() string {
	if e == nil || e.Cause == nil {
		return "acp: start failed after session acceptance"
	}
	return "acp: start failed after session acceptance: " + e.Cause.Error()
}

// Unwrap exposes the post-acceptance cause so failure classification keeps working.
func (e *AcceptedStartError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// AcceptedSessionID reports the accepted ACP session id carried anywhere in err's chain,
// including through fmt.Errorf("%w") and errors.Join wrappers.
func AcceptedSessionID(err error) (string, bool) {
	accepted, ok := errors.AsType[*AcceptedStartError](err)
	if !ok || accepted == nil {
		return "", false
	}
	return accepted.SessionID, true
}

// WrapAcceptedStart marks err as a failure raised after ACP accepted sessionID. The
// driver and the layers above it (session persistence, activation) use it so every chain
// owner derives acceptance from one fact. An error that already carries the acceptance
// fact is returned unchanged.
func WrapAcceptedStart(sessionID string, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := AcceptedSessionID(err); ok {
		return err
	}
	return &AcceptedStartError{SessionID: strings.TrimSpace(sessionID), Cause: err}
}
