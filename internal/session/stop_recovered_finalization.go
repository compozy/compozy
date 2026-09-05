package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/events"
)

func (m *Manager) finishRecoveredStop(ctx context.Context, id string, cause StopCause) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	meta, err := m.readMetaWithContext(cleanupCtx, id)
	if err != nil {
		return err
	}
	snapshot := NotificationSessionFromInfo(m.sessionInfoFromMeta(cleanupCtx, meta))
	sandboxErr := m.finalizeRecoveredSandbox(cleanupCtx, snapshot, &meta)
	m.cancelSessionCompaction(id)
	m.failQueuedSyntheticPrompts(id, ErrSessionNotActive)
	m.clearResumeReplay(id)
	if m.hostedMCP != nil {
		m.hostedMCP.ReleaseSession(id)
	}
	var ledgerErr error
	if cause != CauseClearConversation && cause != CauseConversationRewind {
		ledgerErr = m.rematerializeStoppedSessionLedger(cleanupCtx, id)
		if ledgerErr != nil {
			ledgerErr = errors.Join(ledgerErr, m.recordStoppedCleanupFailure(ctx, snapshot, "ledger", ledgerErr))
		}
	}
	networkErr := m.leaveSessionNetwork(cleanupCtx, snapshot)
	m.dispatchSessionPostStop(cleanupCtx, snapshot)
	if m.notifier != nil {
		m.notifier.OnSessionStopped(cleanupCtx, snapshot)
	}
	return errors.Join(sandboxErr, ledgerErr, networkErr)
}

func (m *Manager) settleRecoveredStop(
	ctx context.Context,
	run *sessionStopRun,
	settlement *recoveredStopSettlement,
) error {
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	outcome := settlement.outcome
	if outcome.Verified {
		if err := m.writeRecoveredStopReceipt(run.recoveredID, settlement); err != nil {
			return err
		}
	}
	if err := m.persistRecoveredStop(settleCtx, run.recoveredID, outcome, settlement.detail, settlement); err != nil {
		return err
	}
	eventType := events.SessionStopVerificationFailed
	if outcome.Verified {
		eventType = EventTypeSessionStopped
	}
	if err := m.recordRecoveredStopEvent(settleCtx, run.recoveredID, settlement, eventType, outcome); err != nil {
		return err
	}
	if !outcome.Verified {
		return nil
	}
	if err := m.removeRecoveredStopReceipt(run.recoveredID); err != nil {
		return err
	}
	m.mu.Lock()
	run.recoveredSettlement = nil
	m.mu.Unlock()
	return m.finishRecoveredStop(ctx, run.recoveredID, outcome.Cause)
}
