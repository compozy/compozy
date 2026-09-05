package session

import (
	"context"
	"errors"
	"fmt"
)

func (m *Manager) finalizeStopped(ctx context.Context, session *Session, waitErr error) error {
	owned, err := m.claimOrWaitFinalization(ctx, session)
	if err != nil || !owned {
		return err
	}
	return m.finalizeStoppedOwned(ctx, session, waitErr, false)
}

func (m *Manager) finalizeObservedStop(
	ctx context.Context,
	session *Session,
	observed *sessionFinalization,
	waitErr error,
) error {
	owned, err := m.claimOrWaitObservedFinalization(ctx, session, observed)
	if err != nil || !owned {
		return err
	}
	return m.finalizeStoppedOwned(ctx, session, waitErr, false)
}

func (m *Manager) finalizeStoppedOwned(
	ctx context.Context,
	session *Session,
	waitErr error,
	promptOwnsTerminalFailure bool,
) (err error) {
	if ctx == nil {
		return errors.New("session: stopped finalization context is required")
	}
	if session == nil {
		return nil
	}
	defer func() { m.finishFinalization(session.ID, err) }()
	if session.pendingStopState() {
		return m.finishStoppedPersistence(ctx, session)
	}

	var errs []error
	errs = appendLifecycleErr(errs, m.beginStoppingSession(ctx, session))
	if verifyErr := m.verifySessionProcessExit(session); verifyErr != nil {
		return errors.Join(append(errs, verifyErr)...)
	}
	classificationErr := m.persistStopClassification(ctx, session, waitErr)
	errs = appendLifecycleErr(errs, classificationErr)
	if errors.Is(classificationErr, ErrRecoveryPersistence) {
		m.dispatchSessionPostStop(ctx, session)
		return errors.Join(errs...)
	}
	if !session.stopProcessExitRecorded {
		processErr := m.recordProcessExitEvent(ctx, session, waitErr, promptOwnsTerminalFailure)
		errs = appendLifecycleErr(errs, processErr)
		session.stopProcessExitRecorded = processErr == nil
	}
	if terminalErr := m.recordSessionStoppedEvent(
		ctx,
		session,
		waitErr,
		promptOwnsTerminalFailure,
	); terminalErr != nil {
		errs = appendLifecycleErr(errs, terminalErr)
		if !session.stopTerminalRecorded {
			m.dispatchSessionPostStop(ctx, session)
			return errors.Join(append(errs, ErrRecoveryPersistence)...)
		}
	}

	errs = appendLifecycleErr(errs, m.finalizeStoppedRuntimeResources(ctx, session, waitErr))
	errs = appendLifecycleErr(errs, m.closeSessionRecorder(session))
	session.stopFinalizationErr = errors.Join(errs...)
	return m.finishStoppedPersistence(ctx, session)
}

func (m *Manager) finishStoppedPersistence(ctx context.Context, session *Session) error {
	if err := m.markSessionStopped(ctx, session); err != nil {
		m.dispatchSessionPostStop(ctx, session)
		return errors.Join(session.stopFinalizationErr, err)
	}
	if err := m.removeRecoveredStopReceipt(session.ID); err != nil {
		session.setPendingStopState(true, true)
		m.dispatchSessionPostStop(ctx, session)
		return errors.Join(session.stopFinalizationErr, err)
	}
	errs := appendLifecycleErr(nil, session.stopFinalizationErr)
	errs = appendLifecycleErr(errs, m.materializeSessionLedger(ctx, session))
	errs = appendLifecycleErr(errs, m.leaveSessionNetwork(ctx, session))
	m.failQueuedSyntheticPrompts(session.ID, ErrSessionNotActive)
	m.clearResumeReplay(session.ID)

	m.removeActive(session.ID)
	if m.hostedMCP != nil {
		m.hostedMCP.ReleaseSession(session.ID)
	}
	m.dispatchSessionPostStop(ctx, session)
	if m.notifier != nil {
		m.notifier.OnSessionStopped(ctx, session)
	}
	if _, healthErr := m.persistSessionStoppedHealth(ctx, session, m.now()); healthErr != nil {
		m.sessionLogger(session).Warn("session: persist stopped health failed", "error", healthErr)
	}
	session.clearProviderSecretRedactions()

	return errors.Join(errs...)
}

// finalizeStoppedRuntimeResources bounds auxiliary I/O independently of the
// terminal persistence context, so cleanup expiry cannot cancel the final write.
func (m *Manager) finalizeStoppedRuntimeResources(ctx context.Context, session *Session, waitErr error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	m.dispatchAgentStopped(cleanupCtx, session, session.processHandle(), waitErr)
	m.logSandboxTransport(session, sandboxEventTransportDisconnect, nil, 0)
	err := m.finalizeSandbox(cleanupCtx, session, sandboxSyncReasonForStop(session))
	m.cancelSessionCompaction(session.ID)
	if notifier, ok := m.notifier.(FinalizationNotifier); ok {
		notifier.OnSessionFinalizing(cleanupCtx, session)
	}
	return err
}

func (m *Manager) leaveSessionNetwork(ctx context.Context, session *Session) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	if err := m.leaveNetworkPeer(cleanupCtx, session); err != nil {
		diagnosticErr := m.recordStoppedCleanupFailure(ctx, session, "network", err)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			m.sessionLogger(session).Warn("session: leave network channel canceled", "error", err)
			return diagnosticErr
		}
		return errors.Join(fmt.Errorf("session: leave network channel for %q: %w", session.ID, err), diagnosticErr)
	}
	return nil
}
