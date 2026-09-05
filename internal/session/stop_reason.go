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
		lowerDetail := strings.ToLower(trimmedDetail)
		switch {
		case strings.Contains(lowerDetail, "max_iterations"):
			return store.StopMaxIterations, trimmedDetail
		case strings.Contains(lowerDetail, "loop_detected"):
			return store.StopLoopDetected, trimmedDetail
		case strings.Contains(lowerDetail, "budget_exceeded"):
			return store.StopBudgetExceeded, trimmedDetail
		default:
			return store.StopUserCanceled, trimmedDetail
		}
	case CauseProcessExited:
		if waitErr != nil {
			return store.StopAgentCrashed, waitErr.Error()
		}
		return store.StopError, "process exited unexpectedly"
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
	case CauseFailed:
		return store.StopError, trimmedDetail
	default:
		if waitErr != nil {
			return store.StopError, waitErr.Error()
		}
		return store.StopCompleted, ""
	}
}

// RequestStopWithCause persists stopping and starts the shared escalation operation.
func (m *Manager) RequestStopWithCause(ctx context.Context, id string, cause StopCause, detail string) error {
	_, err := m.requestSessionStop(ctx, id, cause, detail)
	return err
}

// StopWithCause waits for the same operation used by asynchronous stop requests.
func (m *Manager) StopWithCause(ctx context.Context, id string, cause StopCause, detail string) error {
	previousRun := m.finalizedStopRun(id)
	run, err := m.requestSessionStop(ctx, id, cause, detail)
	if err != nil || run == nil {
		return err
	}
	outcome, err := waitSessionStopRun(ctx, run)
	if previousRun == run && outcome.Verified && outcome.FinalState == StateStopped {
		return nil
	}
	return err
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
	observedProcessExit := isProcessDone(process)
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

	writeMeta, promptSetupDone, err := session.prepareStop(m.now(), appliedCause, appliedDetail)
	if err != nil {
		return nil, nil, false, false, false, fmt.Errorf("session: prepare stop state sync for %q: %w", id, err)
	}
	if writeMeta {
		if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
			return nil, nil, false, false, false, fmt.Errorf(
				"session: prepare stop lifecycle write for %q: %w",
				id,
				err,
			)
		}
	}
	if err := waitForPromptSetup(ctx, session, promptSetupDone); err != nil {
		return nil, nil, false, false, false, fmt.Errorf("session: prepare stop prompt setup wait for %q: %w", id, err)
	}

	if session.Info().State == StateStopped {
		return session, nil, true, stopWasAlreadyRequested, observedProcessExit, nil
	}
	process = session.processHandle()
	return session, process, false, stopWasAlreadyRequested, observedProcessExit, nil
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
