package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) waitForActivePromptProcessExit(ctx context.Context, session *Session) {
	promptDone := session.currentPromptCompletion()
	if promptDone == nil {
		return
	}

	base := ctx
	if base == nil {
		base = m.fallbackLifecycleContext()
	}
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(base), defaultLifecycleTimeout)
	defer cancel()
	select {
	case <-promptDone:
		return
	case <-waitCtx.Done():
		session.cancelCurrentPrompt()
		m.sessionLogger(session).Warn(
			"session: prompt did not release process-exit finalization before timeout",
			"error", waitCtx.Err(),
		)
	}
}

func promptErrorForExitedProcess(proc *AgentProcess, event acp.AgentEvent) acp.AgentEvent {
	if proc == nil || event.Type != acp.EventTypeError {
		return event
	}
	if !isProcessDone(proc) {
		if _, ok := proc.ExitStatus(); !ok {
			return event
		}
	}
	return promptProcessFailureEvent(proc, event)
}

func promptProcessExitEvent(proc *AgentProcess) acp.AgentEvent {
	return promptProcessFailureEvent(proc, acp.AgentEvent{
		Type:  acp.EventTypeError,
		Error: "ACP subprocess exited during active prompt",
	})
}

func promptProcessFailureEvent(proc *AgentProcess, event acp.AgentEvent) acp.AgentEvent {
	const fallback = "ACP subprocess exited during active prompt"

	failureKind := store.FailureProcess
	summary := firstTrimmedNonEmpty(failureSummary(event.Failure, event.Error), event.Error, fallback)
	if proc != nil && isProcessDone(proc) {
		if waitErr := proc.Wait(); waitErr != nil {
			if failure, ok := acp.FailureFromError(waitErr, store.FailureProcess); ok && failure != nil {
				summary = firstTrimmedNonEmpty(failure.Summary, waitErr.Error(), summary)
			} else {
				summary = firstTrimmedNonEmpty(waitErr.Error(), summary)
			}
		}
	}
	if proc != nil {
		if status, ok := proc.ExitStatus(); ok {
			if status.Signal == "" && status.ExitCode == 0 {
				failureKind = store.FailureTransport
			}
			detail := fmt.Sprintf("exit code %d", status.ExitCode)
			if status.Signal != "" {
				detail = "signal " + status.Signal
			}
			summary = fmt.Sprintf("%s (%s)", summary, detail)
		}
	}

	event.Type = acp.EventTypeError
	event.Error = summary
	event.Failure = &store.SessionFailure{
		Kind:    failureKind,
		Summary: summary,
	}
	return event
}
