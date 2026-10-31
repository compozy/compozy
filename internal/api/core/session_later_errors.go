package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func statusForSessionLaterError(err error) (int, bool) {
	switch {
	case errors.Is(err, session.ErrSessionLaterInvalid):
		return http.StatusBadRequest, true
	case errors.Is(err, session.ErrSessionInLater),
		errors.Is(err, store.ErrSessionInLater),
		errors.Is(err, session.ErrSubagentLaterFollowsParent),
		errors.Is(err, session.ErrSessionLaterStopFailed),
		errors.Is(err, store.ErrSessionLaterRequiresStopped):
		return http.StatusConflict, true
	case errors.Is(err, session.ErrSessionLaterUnavailable):
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}

func sessionLaterErrorCode(err error) string {
	switch {
	case errors.Is(err, session.ErrSessionLaterInvalid):
		return "session_later_invalid"
	case errors.Is(err, session.ErrSessionInLater), errors.Is(err, store.ErrSessionInLater):
		return "session_in_later"
	case errors.Is(err, session.ErrSubagentLaterFollowsParent):
		return "subagent_later_follows_parent"
	case errors.Is(err, session.ErrSessionLaterStopFailed), errors.Is(err, store.ErrSessionLaterRequiresStopped):
		return "session_later_stop_failed"
	default:
		return ""
	}
}
