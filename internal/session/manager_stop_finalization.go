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
	defer func() {
		m.finishFinalization(session.ID, err)
		if err == nil && session.Info().State == StateStopped {
			if service := m.subagentService(); service != nil {
				m.logSubagentError(service.OnChildSettled(ctx, session.ID))
			}
		}
	}()
	if session.pendingStopState() {
		return m.finishStoppedPersistence(ctx, session)
	}

	var errs []error
	errs = appendLifecycleErr(errs, m.beginStoppingSession(ctx, session))
	if verifyErr := m.verifySessionProcessExit(session); verifyErr != nil {
		return errors.Join(append(errs, verifyErr)...)
	}
	// A concurrent turn cancellation owns its quiescence receipt until done.
	// Keep the recorder open through that receipt, as the process watcher does.
	if pending := session.pendingTurnStop(session.processHandle()); pending != nil {
		select {
		case <-pending:
		case <-ctx.Done():
			return errors.Join(append(errs, ctx.Err())...)
		}
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

// finishStoppedPersistence retains the receipt until stop history, Goal cancellation
// and supervised task recovery settle.
func (m *Manager) finishStoppedPersistence(ctx context.Context, session *Session) error {
	if err := m.markSessionStopped(ctx, session); err != nil {
		m.dispatchSessionPostStop(ctx, session)
		return errors.Join(session.stopFinalizationErr, err)
	}
	if err := m.stopSessionGoals(ctx, session.Info()); err != nil {
		session.setPendingStopState(true, true)
		m.dispatchSessionPostStop(ctx, session)
		return errors.Join(session.stopFinalizationErr, ErrRecoveryPersistence, err)
	}
	turnID, err := m.sessionStopTurnID(session)
	if err == nil {
		err = m.recoverSupervisedWork(ctx, session.Info(), turnID)
	}
	if err != nil {
		session.setPendingStopState(true, true)
		return errors.Join(session.stopFinalizationErr, ErrRecoveryPersistence, err)
	}
	if err := m.removeRecoveredStopReceipt(session.ID); err != nil {
		session.setPendingStopState(true, true)
		m.dispatchSessionPostStop(ctx, session)
		return errors.Join(session.stopFinalizationErr, err)
	}
	errs := appendLifecycleErr(nil, session.stopFinalizationErr)
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
	if notifier, ok := m.notifier.(FinalizationNotifier); ok {
		notifier.OnSessionFinalizing(cleanupCtx, session)
	}
	return nil
}
