package session

import "github.com/compozy/compozy/internal/store"

func classifyDaemonInterruption(meta, next *store.SessionMeta, state State) bool {
	resuming := state == StateStarting && sessionMetaStopReason(meta) == store.StopShutdown &&
		meta.ACPSessionID != nil && meta.AcceptedRoute != nil
	if (state != StateActive && state != StateStopping && !resuming) || meta.Failure != nil {
		return false
	}
	// Ready runtimes retain the last completed binding strategy (including initial_bind).
	// Only an unfinished transition changes recovery eligibility.
	if meta.RuntimeStatus != store.SessionRuntimeReady && !resuming &&
		meta.RuntimeTransition != store.SessionRuntimeTransitionNone {
		return false
	}
	reason := sessionMetaStopReason(meta)
	if state == StateActive && reason != "" && reason != store.StopShutdown {
		return false
	}
	next.State = string(StateStopped)
	next.StopVerificationFailed = !inactiveProcessExitVerified(meta)
	if next.StopVerificationFailed {
		next.State = string(StateStopping)
	}
	if reason == "" || reason == store.StopShutdown {
		next.StopReason = new(store.StopShutdown)
		next.StopDetail = "Interrupted by daemon restart."
	}
	return true
}
