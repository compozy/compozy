package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const stopDetailDaemonShutdown = "daemon shutdown"

func classifyStopReason(cause StopCause, waitErr error, detail string) (store.StopReason, string) {
	trimmedDetail := strings.TrimSpace(detail)

	switch cause {
	case CauseShutdown:
		if trimmedDetail == "" {
			trimmedDetail = stopDetailDaemonShutdown
		}
		return store.StopShutdown, trimmedDetail
	case CauseHookDenied:
		return store.StopHookStopped, trimmedDetail
	case CauseUserRequested:
		return classifyUserRequestedStopReason(trimmedDetail), trimmedDetail
	case CauseProcessExited:
		if waitErr != nil {
			if failure := sessionFailureFromError(waitErr, store.FailureProcess); failure.Kind == store.FailurePrompt {
				return store.StopError, failure.Summary
			}
			return store.StopAgentCrashed, waitErr.Error()
		}
		return store.StopError, "process exited unexpectedly"
	case CauseInactivity:
		return store.StopTimeout, stopDetailInactivity
	case CauseTimeout:
		if trimmedDetail == "" {
			trimmedDetail = store.SessionStallReasonActivityTimeout
		}
		return store.StopTimeout, trimmedDetail
	case CauseClearConversation:
		if trimmedDetail == "" {
			trimmedDetail = "conversation cleared"
		}
		return store.StopCompleted, trimmedDetail
	case CauseConversationRewind:
		if trimmedDetail == "" {
			trimmedDetail = "conversation rewound"
		}
		return store.StopCompleted, trimmedDetail
	case CauseCompleted:
		return store.StopCompleted, trimmedDetail
	case CauseOwnerReleased:
		return store.StopOwnerReleased, trimmedDetail
	case CauseFailed:
		return store.StopError, trimmedDetail
	default:
		if waitErr != nil {
			return store.StopError, waitErr.Error()
		}
		return store.StopCompleted, ""
	}
}

func classifyUserRequestedStopReason(detail string) store.StopReason {
	lowerDetail := strings.ToLower(detail)
	switch {
	case strings.Contains(lowerDetail, "max_iterations"):
		return store.StopMaxIterations
	case strings.Contains(lowerDetail, "loop_detected"):
		return store.StopLoopDetected
	case strings.Contains(lowerDetail, "budget_exceeded"):
		return store.StopBudgetExceeded
	default:
		return store.StopUserCanceled
	}
}

// RequestStopWithCause persists stopping and starts the shared escalation operation.
func (m *Manager) RequestStopWithCause(ctx context.Context, id string, cause StopCause, detail string) error {
	_, err := m.requestSessionStop(ctx, id, cause, detail)
	if err == nil && cause != CauseShutdown && cause != CauseProcessExited {
		if service := m.subagentService(); service != nil {
			err = service.OnParentStopped(ctx, id)
		}
	}
	return err
}

// StopWithCause waits for the same operation used by asynchronous stop requests.
func (m *Manager) StopWithCause(ctx context.Context, id string, cause StopCause, detail string) error {
	_, err := m.stopWithCauseReceipt(ctx, id, cause, detail)
	return err
}

func (m *Manager) stopWithCauseReceipt(ctx context.Context, id string, cause StopCause, detail string) (bool, error) {
	previousRun := m.finalizedStopRun(id)
	run, err := m.requestSessionStop(ctx, id, cause, detail)
	if err != nil || run == nil {
		return false, err
	}
	if service := m.subagentService(); service != nil && cause != CauseShutdown && cause != CauseProcessExited {
		if cascadeErr := service.OnParentStopped(ctx, id); cascadeErr != nil {
			m.logger.ErrorContext(ctx, "subagent.stop_cascade", "session_id", id, "error", cascadeErr)
		}
	}
	outcome, err := waitSessionStopRun(ctx, run)
	if previousRun == run && outcome.Verified && outcome.FinalState == StateStopped {
		err = nil
	}
	if err == nil {
		if service := m.subagentService(); service != nil {
			err = service.OnChildSettled(ctx, id)
		}
	}
	return true, err
}

func (m *Manager) finalizedStopRun(id string) *sessionStopRun {
	if m == nil {
		return nil
	}
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.sessions[id] != nil || m.finalizing[id] != nil {
		return nil
	}
	run := m.stopRuns[id]
	if run == nil || !signalClosed(run.done) {
		return nil
	}
	return run
}

// StopWithSpawnTTL stops a spawned session and classifies its prompt state
// atomically with the lifecycle transition.
func (m *Manager) StopWithSpawnTTL(ctx context.Context, id string, detail string) error {
	return m.StopWithCause(ctx, id, CauseSpawnTTLExpired, detail)
}

func (m *Manager) prepareStopWithCause(
	ctx context.Context,
	id string,
	cause StopCause,
	detail string,
) (*Session, *AgentProcess, bool, bool, bool, error) {
	return m.prepareStopWithCauseMode(ctx, id, cause, detail, stopPreparationDefault)
}

type stopPreparationMode uint8

const (
	stopPreparationDefault stopPreparationMode = iota
	stopPreparationFatalPromptFailure
)

func (m *Manager) prepareStopWithCauseMode(
	ctx context.Context,
	id string,
	cause StopCause,
	detail string,
	mode stopPreparationMode,
) (*Session, *AgentProcess, bool, bool, bool, error) {
	session, finalization, err := m.stopTarget(id)
	if err != nil {
		return nil, nil, false, false, false, err
	}
	if finalization != nil {
		if finalizationErr := waitForSessionFinalization(ctx, finalization); finalizationErr != nil {
			return nil, nil, false, false, false, fmt.Errorf(
				"session: wait for finalization of %q: %w",
				id,
				finalizationErr,
			)
		}
		return session, nil, true, false, true, nil
	}
	process := session.processHandle()
	stopWasAlreadyRequested := session.stopWasRequested()
	// An unbound session has no process yet; absence is not a natural exit.
	observedProcessExit := process != nil && isProcessDone(process)
	fatalPromptFailure := mode == stopPreparationFatalPromptFailure

	appliedCause := cause
	appliedDetail := detail
	if observedProcessExit && !stopWasAlreadyRequested && !fatalPromptFailure {
		appliedCause = CauseNone
		appliedDetail = ""
	}

	if session.Info().State == StateActive && (!observedProcessExit || fatalPromptFailure) {
		if err := m.dispatchSessionPreStop(ctx, session); err != nil {
			return nil, nil, false, false, false, fmt.Errorf("session: prepare stop pre-stop hooks for %q: %w", id, err)
		}
	}

	promptSetupDone, err := m.prepareStopStateAndGoals(ctx, session, appliedCause, appliedDetail)
	if err != nil {
		return nil, nil, false, false, false, err
	}
	if err := waitForPromptSetup(ctx, session, promptSetupDone); err != nil {
		return nil, nil, false, false, false, fmt.Errorf("session: prepare stop prompt setup wait for %q: %w", id, err)
	}

	if session.Info().State == StateStopped && !session.pendingStopState() {
		return session, nil, true, stopWasAlreadyRequested, observedProcessExit, nil
	}
	process = session.processHandle()
	return session, process, false, stopWasAlreadyRequested, observedProcessExit, nil
}

func (m *Manager) prepareStopStateAndGoals(
	ctx context.Context, session *Session, cause StopCause, detail string,
) (<-chan struct{}, error) {
	// An in-flight setup can observe stopping before the durable pause reaches its
	// Goal. Keep its ambiguity settlement behind that intent, then unlock before
	// waiting for setup to drain so neither side waits on the other.
	session.goalStopMu.Lock()
	defer session.goalStopMu.Unlock()
	writeMeta, promptSetupDone, err := session.prepareStop(m.now(), cause, detail)
	if err != nil {
		return nil, fmt.Errorf("session: prepare stop state sync for %q: %w", session.ID, err)
	}
	if writeMeta {
		if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
			return nil, fmt.Errorf(
				"session: prepare stop lifecycle write for %q: %w",
				session.ID,
				err,
			)
		}
	}
	if err := m.stopSessionGoals(ctx, session.Info()); err != nil {
		// Verified stop finalization retries this durable intent before releasing its receipt.
		m.sessionLogger(session).Warn("session: prepare Goal pause failed", "error", err)
	}
	return promptSetupDone, nil
}

func reconcileObservedTerminalStop(session *Session, stopWasAlreadyRequested bool, waitErr error) {
	if session == nil || stopWasAlreadyRequested {
		return
	}
	if waitErr != nil {
		session.setStopCause(CauseProcessExited)
		return
	}
	session.setStopCause(CauseCompleted)
}
