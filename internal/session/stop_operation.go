package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type sessionStopRun struct {
	session             *Session
	ready               chan struct{}
	done                chan struct{}
	prepareErr          error
	recoveredID         string
	recoveredSettlement *recoveredStopSettlement
	outcome             StopOutcome
	err                 error
	// guard and onCommitted belong to the claiming caller. stopCommitted records that
	// the guard admitted the stop; joiners read it only after ready closes.
	guard         stopGuard
	onCommitted   func()
	stopCommitted bool
}

// RequestStop starts or joins a session stop without waiting for termination.
func (m *Manager) RequestStop(ctx context.Context, id string, cause StopCause) error {
	return m.RequestStopWithCause(ctx, id, cause, "")
}

// AwaitStopped waits for the shared operation, including an unverified outcome.
func (m *Manager) AwaitStopped(ctx context.Context, id string) (StopOutcome, error) {
	if m == nil || ctx == nil {
		return StopOutcome{}, errors.New("session: stop wait requires manager and context")
	}
	m.mu.RLock()
	run := m.stopRuns[strings.TrimSpace(id)]
	active := m.sessions[strings.TrimSpace(id)]
	m.mu.RUnlock()
	if run == nil || (active != nil && run.session != active) {
		return StopOutcome{}, fmt.Errorf("session: no stop requested for %q", id)
	}
	return waitSessionStopRun(ctx, run)
}

func waitSessionStopRun(ctx context.Context, run *sessionStopRun) (StopOutcome, error) {
	select {
	case <-run.done:
		if errors.Is(run.err, ErrStopVerificationFailed) {
			return run.outcome, stopVerificationError{run.err}
		}
		return run.outcome, run.err
	case <-ctx.Done():
		return StopOutcome{}, ctx.Err()
	}
}

func (m *Manager) requestSessionStop(
	ctx context.Context, id string, cause StopCause, detail string,
) (*sessionStopRun, error) {
	return m.requestGuardedSessionStop(ctx, id, cause, detail, nil, nil)
}

// requestGuardedSessionStop starts or joins a stop. A guarded stop commits only
// if guard still holds; onCommitted then runs before termination begins.
func (m *Manager) requestGuardedSessionStop(
	ctx context.Context, id string, cause StopCause, detail string, guard stopGuard, onCommitted func(),
) (*sessionStopRun, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: stop context is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("session: session id is required")
	}
	if cause == CauseNone {
		cause = CauseUserRequested
	}
	if err := m.waitForConversationFinalization(ctx, id); err != nil {
		return nil, err
	}
	run, owned := m.claimSessionStopRun(id)
	for !owned {
		select {
		case <-run.ready:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if !run.uncommittedGuardedStop() {
			return run, run.prepareErr
		}
		// That run stopped nothing, and its guard spoke only for its own caller.
		run, owned = m.claimSessionStopRun(id)
	}
	run.guard, run.onCommitted = guard, onCommitted
	if run.session == nil {
		m.prepareRecoveredStopRun(ctx, run, cause, detail)
		return run, run.prepareErr
	}
	if !run.outcome.Verified && m.prepareStartingStopRun(ctx, run, cause, detail) {
		return run, run.prepareErr
	}
	m.prepareSessionStopRun(ctx, run, cause, detail)
	return run, run.prepareErr
}

func (m *Manager) claimSessionStopRun(id string) (*sessionStopRun, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	active := m.sessions[id]
	if active == nil && m.finalizing[id] != nil {
		active = m.finalizing[id].session
	}
	var settlement *recoveredStopSettlement
	var verifiedOutcome StopOutcome
	if previous := m.stopRuns[id]; previous != nil &&
		(!signalClosed(previous.ready) || active == nil || previous.session == active) {
		select {
		case <-previous.done:
			if previous.outcome.Verified && previous.outcome.FinalState == StateStopped &&
				previous.recoveredSettlement == nil && !errors.Is(previous.err, ErrRecoveryPersistence) {
				return previous, false
			}
			if previous.outcome.Verified {
				verifiedOutcome = previous.outcome
			}
			settlement = previous.recoveredSettlement
		default:
			return previous, false
		}
	}
	run := &sessionStopRun{
		session: active,
		ready:   make(chan struct{}),
		done:    make(chan struct{}),
		outcome: verifiedOutcome,
	}
	if active == nil {
		run.recoveredID = id
		run.recoveredSettlement = settlement
		if settlement != nil {
			run.outcome = settlement.outcome
		}
	}
	if m.stopRuns == nil {
		m.stopRuns = make(map[string]*sessionStopRun)
	}
	m.stopRuns[id] = run
	return run, true
}

// failStopPreparation ends a run whose preparation failed. A committed stop still
// owes its callback. An uncommitted guarded run leaves no trace, so it can read
// neither as a stop in progress nor as a stop result.
func (m *Manager) failStopPreparation(run *sessionStopRun, err error) {
	run.prepareErr, run.err = err, err
	if run.stopCommitted {
		run.notifyCommitted()
	}
	if run.uncommittedGuardedStop() {
		m.mu.Lock()
		if m.stopRuns[run.session.ID] == run {
			delete(m.stopRuns, run.session.ID)
		}
		m.mu.Unlock()
	}
	close(run.ready)
	close(run.done)
}

func (r *sessionStopRun) uncommittedGuardedStop() bool {
	return r.guard != nil && !r.stopCommitted
}

// notifyCommitted runs the callback at most once. Nil receivers are unguarded stops.
func (r *sessionStopRun) notifyCommitted() {
	if r == nil || r.onCommitted == nil {
		return
	}
	notify := r.onCommitted
	r.onCommitted = nil
	notify()
}

func (m *Manager) prepareSessionStopRun(ctx context.Context, run *sessionStopRun, cause StopCause, detail string) {
	session, proc, stopped, requested, _, err := m.prepareStopWithCauseMode(
		ctx, run.session.ID, cause, detail, stopPreparationDefault, run,
	)
	if err != nil {
		m.failStopPreparation(run, err)
		return
	}
	if !run.outcome.Verified {
		run.outcome.Cause = cause
	}
	run.notifyCommitted()
	close(run.ready)
	if stopped {
		run.outcome.FinalState, run.outcome.Verified = StateStopped, true
		close(run.done)
		return
	}
	observed := m.observeFinalization(session)
	promptDone := session.currentPromptCompletion()
	m.startTrackedPromptTask(func() {
		defer close(run.done)
		result := terminationResult{StopOutcome: run.outcome}
		var ladderErr error
		if !result.Verified {
			stopProc := proc
			if stopProc == nil {
				stopProc = session.releasedProcessForStop()
			}
			target := terminationTarget{
				promptDone: promptDone,
				beforeAction: func(ctx context.Context, phase StopPhase, elapsed time.Duration) error {
					return m.recordSessionStopEscalation(ctx, session, phase, elapsed, cause)
				},
			}
			if (cause == CauseCompleted || cause == CauseOwnerReleased) &&
				(promptDone == nil || signalClosed(promptDone)) {
				target.cooperative = m.driver.Stop
			}
			result, ladderErr = m.runTerminationLadder(ctx, stopProc, target)
		}
		outcome := result.StopOutcome
		outcome.Cause = cause
		run.outcome, run.err = outcome, errors.Join(ladderErr, result.observationErr)
		if !run.outcome.Verified {
			run.err = errors.Join(
				run.err,
				m.recordSessionStopVerificationFailure(context.WithoutCancel(ctx), session, outcome),
			)
			owned, waitErr := m.claimOrWaitObservedFinalization(context.WithoutCancel(ctx), session, observed)
			if owned {
				m.finishFinalization(session.ID, run.err)
			}
			run.err = errors.Join(run.err, waitErr)
			return
		}
		run.outcome = session.retainVerifiedStopOutcome(run.outcome)
		var waitErr error
		if proc != nil && isProcessDone(proc) {
			waitErr = proc.Wait()
		}
		reconcileObservedTerminalStop(session, requested, waitErr)
		run.err = errors.Join(run.err, m.finalizeObservedStop(context.WithoutCancel(ctx), session, observed, waitErr))
		run.outcome.FinalState = session.Info().State
		if errors.Is(run.err, ErrStopVerificationFailed) {
			run.outcome.Verified = false
		}
		run.outcome.Cause, _ = session.stopCauseDetail()
	})
}
