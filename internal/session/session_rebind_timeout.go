package session

import (
	"errors"

	"github.com/compozy/compozy/internal/diagnostics"
)

// SessionRebindTimeoutCode names the retryable load-and-replay refusal on public transports.
const SessionRebindTimeoutCode = "session_rebind_timeout"

// SessionRebindTimeoutError reports that loading a released provider session timed out
// and the transcript-replay fallback could not start.
type SessionRebindTimeoutError struct {
	LoadTimeout error
	Fallback    error
}

var _ error = (*SessionRebindTimeoutError)(nil)

func (e *SessionRebindTimeoutError) Error() string {
	return "The provider session took too long to reload. Retry, or use Continue/Fork to keep " +
		"the transcript on a fresh provider thread. Your message wasn't sent. (" +
		SessionRebindTimeoutCode + ")"
}

func (e *SessionRebindTimeoutError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return sessionRebindTimeoutCauses(e.LoadTimeout, e.Fallback)
}

// ProviderError returns redacted, bounded details about the timed-out load and failed replay.
func (e *SessionRebindTimeoutError) ProviderError() string {
	if e == nil {
		return ""
	}
	cause := errors.Join(e.LoadTimeout, e.Fallback)
	if cause == nil {
		return ""
	}
	return diagnostics.RedactAndBound(cause.Error(), 2048)
}

func sessionRebindTimeoutCauses(errs ...error) []error {
	result := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
