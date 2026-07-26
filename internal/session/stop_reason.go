package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
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
	case CauseCompleted:
		return store.StopCompleted, ""
	case CauseFailed:
		return store.StopError, trimmedDetail
	default:
		if waitErr != nil {
			return store.StopError, waitErr.Error()
		}
		return store.StopCompleted, ""
	}
}

// RequestStopWithCause marks a session as stopping and sends the cooperative ACP
// cancel signal without forcing process termination.
func (m *Manager) RequestStopWithCause(ctx context.Context, id string, cause StopCause, detail string) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: request stop context is required")
	}
	if cause == CauseNone {
		cause = CauseUserRequested
	}
	if handled, err := m.stopStartingSession(ctx, id, cause, detail); handled {
		return err
	}

	session, proc, alreadyStopped, stopWasAlreadyRequested, observedProcessExit, err := m.prepareStopWithCause(
		ctx,
		id,
		cause,
		detail,
	)
	if err != nil {
		return err
	}
	if alreadyStopped {
		return nil
	}
	if proc == nil {
		return m.finalizeStopped(ctx, session, nil)
	}
	if observedProcessExit {
		waitErr := proc.Wait()
		reconcileObservedTerminalStop(session, stopWasAlreadyRequested, waitErr)

		finalizeErr := m.finalizeStopped(ctx, session, waitErr)
		if finalizeErr != nil {
			return finalizeErr
		}
		return nil
	}

	cancelErr := m.driver.Cancel(ctx, proc)
	if cancelErr != nil && !isProcessDone(proc) {
		return fmt.Errorf("session: request cooperative stop for %q: %w", id, cancelErr)
	}
	if isProcessDone(proc) {
		waitErr := proc.Wait()
		reconcileObservedTerminalStop(session, stopWasAlreadyRequested, waitErr)

		finalizeErr := m.finalizeStopped(ctx, session, waitErr)
		if finalizeErr != nil {
			return errors.Join(cancelErr, finalizeErr)
		}
		return nil
	}
	return cancelErr
}

// StopWithCause stops a session while preserving the explicit stop initiator.
func (m *Manager) StopWithCause(ctx context.Context, id string, cause StopCause, detail string) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: stop context is required")
	}
	if cause == CauseNone {
		cause = CauseUserRequested
	}
	if handled, err := m.stopStartingSession(ctx, id, cause, detail); handled {
		return err
	}

	session, proc, alreadyStopped, stopWasAlreadyRequested, observedProcessExit, err := m.prepareStopWithCause(
		ctx,
		id,
		cause,
		detail,
	)
	if err != nil {
		return err
	}
	if alreadyStopped {
		return nil
	}
	if proc == nil {
		return m.finalizeStopped(ctx, session, nil)
	}
	if observedProcessExit {
		waitErr := proc.Wait()
		reconcileObservedTerminalStop(session, stopWasAlreadyRequested, waitErr)

		return m.finalizeStopped(ctx, session, waitErr)
	}

	finalization := m.observeFinalization(session)
	doneBeforeStop := isProcessDone(proc)
	stopErr := m.driver.Stop(ctx, proc)
	doneAfterStop := isProcessDone(proc)
	if stopErr == nil && !doneAfterStop {
		select {
		case <-proc.Done():
			doneAfterStop = true
		case <-ctx.Done():
			waitErr := fmt.Errorf("session: wait for process stop completion for %q: %w", id, ctx.Err())
			m.finishFinalization(session.ID, waitErr)
			return waitErr
		}
	}
	if stopErr != nil {
		if !doneBeforeStop && !doneAfterStop {
			stopErr = fmt.Errorf("session: stop session process for %q: %w", id, stopErr)
			m.finishFinalization(session.ID, stopErr)
			return stopErr
		}
		stopErr = nil
	}

	waitErr := proc.Wait()
	reconcileObservedTerminalStop(session, stopWasAlreadyRequested, waitErr)

	finalizeErr := m.finalizeObservedStop(ctx, session, finalization, waitErr)
	if finalizeErr != nil {
		return errors.Join(stopErr, finalizeErr)
	}
	return nil
}

func (m *Manager) prepareStopWithCause(
	ctx context.Context,
	id string,
	cause StopCause,
	detail string,
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

	appliedCause := cause
	appliedDetail := detail
	if observedProcessExit && !stopWasAlreadyRequested {
		appliedCause = CauseNone
		appliedDetail = ""
	}

	if session.Info().State == StateActive && !observedProcessExit {
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
