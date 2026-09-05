package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

type recoveredStopSettlement struct {
	turnID    string
	startedAt time.Time
	actorID   string
	outcome   StopOutcome
	detail    string
	receipt   recoveredStopReceipt
}

func (m *Manager) hasPendingStopSettlement(id string) bool {
	m.mu.RLock()
	run := m.stopRuns[id]
	active := m.sessions[id]
	pending := run != nil && signalClosed(run.done) && errors.Is(run.err, ErrRecoveryPersistence)
	recovered := run != nil && run.recoveredID == id &&
		((signalClosed(run.ready) && !signalClosed(run.done)) || run.recoveredSettlement != nil)
	m.mu.RUnlock()
	return pending || recovered || (active != nil && active.pendingStopState())
}

func (m *Manager) prepareRecoveredStopRun(
	ctx context.Context, run *sessionStopRun, cause StopCause, detail string,
) {
	id := run.recoveredID
	operationCtx, unlock := ctx, func() {}
	if !ownsConversationOperation(ctx, id) {
		var err error
		operationCtx, unlock, err = m.lockConversationOperation(ctx, id)
		if err != nil {
			m.failRecoveredStopPreparation(run, err)
			return
		}
	}
	if active, ok := m.Get(id); ok {
		m.mu.Lock()
		run.session, run.recoveredID, run.recoveredSettlement = active, "", nil
		m.mu.Unlock()
		unlock()
		if !m.prepareStartingStopRun(ctx, run, cause, detail) {
			m.prepareSessionStopRun(ctx, run, cause, detail)
		}
		return
	}
	meta, err := m.readMetaWithContext(operationCtx, id)
	if err != nil {
		unlock()
		m.failRecoveredStopPreparation(run, err)
		return
	}
	if run.recoveredSettlement == nil && meta.State != string(StateStopping) && meta.State != string(StateStopped) {
		unlock()
		m.failRecoveredStopPreparation(
			run,
			fmt.Errorf("%w: recovered session %s is %s", ErrSessionNotActive, id, meta.State),
		)
		return
	}
	if settlement := run.recoveredSettlement; settlement != nil {
		run.outcome = settlement.outcome
		close(run.ready)
		m.startTrackedPromptTask(func() {
			defer unlock()
			defer close(run.done)
			run.err = m.settleRecoveredStop(context.WithoutCancel(operationCtx), run, settlement)
		})
		return
	}
	run.outcome.Cause = cause
	close(run.ready)
	if meta.State == string(StateStopped) {
		run.outcome.FinalState, run.outcome.Verified = StateStopped, true
		close(run.done)
		unlock()
		return
	}
	m.startTrackedPromptTask(func() {
		defer unlock()
		defer close(run.done)
		m.runRecoveredStop(context.WithoutCancel(operationCtx), run, &meta, cause, detail)
	})
}

func (m *Manager) failRecoveredStopPreparation(run *sessionStopRun, err error) {
	run.prepareErr, run.err = err, err
	m.mu.Lock()
	if m.stopRuns[run.recoveredID] == run && run.recoveredSettlement == nil {
		delete(m.stopRuns, run.recoveredID)
	}
	m.mu.Unlock()
	close(run.ready)
	close(run.done)
}

func (m *Manager) runRecoveredStop(
	ctx context.Context, run *sessionStopRun, meta *store.SessionMeta, cause StopCause, detail string,
) {
	turnID, idErr := m.newPromptTurnID()
	if idErr != nil {
		run.err = idErr
		run.outcome.FinalState = StateStopping
		return
	}
	settlement := &recoveredStopSettlement{
		turnID:    turnID,
		startedAt: m.now(),
		actorID:   actingSessionID(ctx),
		detail:    detail,
		receipt: recoveredStopReceipt{
			Version: 1, SessionID: meta.ID, WorkspaceID: meta.WorkspaceID,
			RuntimeGeneration: meta.RuntimeGeneration, CreatedAt: meta.CreatedAt,
		},
	}
	proc, target := m.recoveredTerminationTarget(meta)
	target.beforeAction = func(ctx context.Context, phase StopPhase, elapsed time.Duration) error {
		outcome := StopOutcome{
			FinalState: StateStopping, Escalated: true, Phase: phase, Cause: cause, Elapsed: elapsed,
		}
		return errors.Join(m.persistRecoveredStop(ctx, run.recoveredID, outcome, detail, nil),
			m.recordRecoveredStopEvent(ctx, run.recoveredID, settlement, events.SessionStopEscalated, outcome))
	}
	result, err := m.runTerminationLadder(ctx, proc, target)
	run.outcome = result.StopOutcome
	run.outcome.Cause = cause
	if run.outcome.Verified {
		run.outcome.FinalState = StateStopped
	}
	settlement.outcome = run.outcome
	if run.outcome.Verified {
		m.mu.Lock()
		run.recoveredSettlement = settlement
		m.mu.Unlock()
	}
	run.err = errors.Join(err, result.observationErr, m.settleRecoveredStop(ctx, run, settlement))
}

func (m *Manager) persistRecoveredStop(
	ctx context.Context, id string, outcome StopOutcome, detail string, settlement *recoveredStopSettlement,
) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: persist recovered stop for %s: %w", ErrRecoveryPersistence, id, err)
		}
	}()
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return err
	}
	meta.StopEscalated = meta.StopEscalated || outcome.Escalated
	if settlement != nil {
		meta.StopVerificationFailed = !outcome.Verified
		if outcome.Verified {
			if interruptedStartupMeta(&meta) {
				meta.ACPSessionID = nil
				meta.Failure = interruptedSessionFailure(meta.Failure, store.FailureStartup, detail)
			}
			meta.State = string(StateStopped)
			if settlement.receipt.TerminalEvent == nil {
				reason, stopDetail := classifyStopReason(outcome.Cause, nil, detail)
				if outcome.Cause == CauseProcessExited {
					reason, stopDetail = store.StopAgentCrashed, detail
					if sessionMetaStopReason(&meta) == store.StopAgentCrashed {
						stopDetail = meta.StopDetail
					}
					meta.Failure = interruptedSessionFailure(meta.Failure, store.FailureProcess, stopDetail)
				}
				meta.StopReason, meta.StopDetail = &reason, stopDetail
			}
		}
	}
	meta.UpdatedAt = m.now().UTC()
	path := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, id))
	if err := store.WriteSessionMeta(path, meta); err != nil {
		return err
	}
	return m.persistRecoveryCatalog(ctx, &meta)
}
