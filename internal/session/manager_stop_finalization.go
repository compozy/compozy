package session

import (
	"context"
	"errors"
)

func (m *Manager) finalizeStopped(ctx context.Context, session *Session, waitErr error) error {
	owned, err := m.claimOrWaitFinalization(ctx, session)
	if err != nil || !owned {
		return err
	}
	return m.finalizeStoppedOwned(ctx, session, waitErr)
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
	return m.finalizeStoppedOwned(ctx, session, waitErr)
}

func (m *Manager) finalizeStoppedOwned(
	ctx context.Context,
	session *Session,
	waitErr error,
) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if session == nil {
		return nil
	}
	defer func() { m.finishFinalization(session.ID, err) }()

	var errs []error
	errs = appendLifecycleErr(errs, m.beginStoppingSession(ctx, session))
	errs = appendLifecycleErr(errs, m.persistStopClassification(ctx, session, waitErr))
	errs = appendLifecycleErr(errs, m.recordProcessExitEvent(ctx, session, waitErr))
	errs = appendLifecycleErr(errs, m.recordSessionStoppedEvent(ctx, session, waitErr))

	m.dispatchAgentStopped(ctx, session, session.processHandle(), waitErr)
	m.logSandboxTransport(session, sandboxEventTransportDisconnect, nil, 0)
	errs = appendLifecycleErr(errs, m.finalizeSandbox(ctx, session, sandboxSyncReasonForStop(session)))

	compactionErr := m.cancelAndWaitSessionCompaction(ctx, session.ID)
	errs = appendLifecycleErr(errs, compactionErr)
	if compactionErr != nil {
		return errors.Join(errs...)
	}
	if notifier, ok := m.notifier.(FinalizationNotifier); ok {
		notifier.OnSessionFinalizing(ctx, session)
	}
	errs = appendLifecycleErr(errs, m.closeSessionRecorder(session))
	errs = appendLifecycleErr(errs, m.markSessionStopped(ctx, session))
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
