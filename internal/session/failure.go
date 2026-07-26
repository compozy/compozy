package session

import (
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
)

const maxSessionFailureSummaryBytes = 2048

func sessionFailureForStop(cause StopCause, waitErr error, detail string) *store.SessionFailure {
	if waitErr != nil {
		if failure, ok := acp.FailureFromError(waitErr, store.FailureProcess); ok {
			return normalizeSessionFailure(failure, waitErr.Error())
		}
		return normalizeSessionFailure(&store.SessionFailure{
			Kind:    store.FailureProcess,
			Summary: waitErr.Error(),
		}, waitErr.Error())
	}

	switch cause {
	case CauseUserRequested:
		return normalizeSessionFailure(&store.SessionFailure{
			Kind:    store.FailureCanceled,
			Summary: firstNonEmptySessionFailureText(detail, "session canceled by user"),
		}, "")
	case CauseTimeout:
		return normalizeSessionFailure(&store.SessionFailure{
			Kind:    store.FailureTimeout,
			Summary: firstNonEmptySessionFailureText(detail, "session timed out"),
		}, "")
	case CauseHookDenied:
		return normalizeSessionFailure(&store.SessionFailure{
			Kind:    store.FailurePermission,
			Summary: firstNonEmptySessionFailureText(detail, "session stopped by hook policy"),
		}, "")
	case CauseFailed:
		return normalizeSessionFailure(&store.SessionFailure{
			Kind:    store.FailureUnknown,
			Summary: firstNonEmptySessionFailureText(detail, "session failed"),
		}, "")
	default:
		return nil
	}
}

func sessionFailureFromError(err error, fallback store.FailureKind) *store.SessionFailure {
	if err == nil {
		return nil
	}
	if failure, ok := acp.FailureFromError(err, fallback); ok {
		return normalizeSessionFailure(failure, err.Error())
	}
	return normalizeSessionFailure(&store.SessionFailure{
		Kind:    fallback,
		Summary: err.Error(),
	}, err.Error())
}

func normalizeSessionFailure(failure *store.SessionFailure, fallbackSummary string) *store.SessionFailure {
	if failure == nil {
		return nil
	}
	normalized := failure.Normalize()
	if normalized.Kind == "" {
		normalized.Kind = store.FailureUnknown
	}
	if normalized.Summary == "" {
		normalized.Summary = strings.TrimSpace(fallbackSummary)
	}
	normalized.Summary = diagnostics.RedactAndBound(normalized.Summary, maxSessionFailureSummaryBytes)
	normalized.CrashBundlePath = diagnostics.RedactAndBound(normalized.CrashBundlePath, maxSessionFailureSummaryBytes)
	if normalized.IsZero() {
		return nil
	}
	return &normalized
}

func failureStopReason(failure *store.SessionFailure) store.StopReason {
	if failure == nil {
		return store.StopError
	}
	switch failure.Kind {
	case store.FailureCanceled:
		return store.StopUserCanceled
	case store.FailureTimeout:
		return store.StopTimeout
	case store.FailureProcess:
		return store.StopAgentCrashed
	default:
		return store.StopError
	}
}

func failureSummary(failure *store.SessionFailure, fallback string) string {
	if failure != nil {
		if summary := strings.TrimSpace(failure.Summary); summary != "" {
			return summary
		}
	}
	return diagnostics.RedactAndBound(fallback, maxSessionFailureSummaryBytes)
}

func firstNonEmptySessionFailureText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
