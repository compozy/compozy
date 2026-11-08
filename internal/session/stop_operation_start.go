package session

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// prepareStartingStopRun joins startup cancellation without releasing its terminal cleanup owner.
func (m *Manager) prepareStartingStopRun(
	ctx context.Context,
	run *sessionStopRun,
	cause StopCause,
	detail string,
) bool {
	session, start, handled, err := m.prepareStartingSessionStop(ctx, run.session.ID, cause, detail, run)
	if !handled {
		return false
	}
	if err != nil {
		m.failStopPreparation(run, err)
		return true
	}
	m.mu.Lock()
	run.session = session
	m.mu.Unlock()
	run.notifyCommitted()
	close(run.ready)
	m.startTrackedPromptTask(func() {
		defer close(run.done)
		started := m.now()
		run.err = m.finishStartingSessionStop(context.WithoutCancel(ctx), session, start)
		info := session.Info()
		run.outcome = StopOutcome{
			FinalState: info.State, Verified: info.State == StateStopped && (start == nil || signalClosed(start.done)),
			Cause: cause, Phase: StopPhaseCooperative, Elapsed: m.now().Sub(started),
		}
		if start != nil && signalClosed(start.done) && start.stopOutcome != nil {
			run.outcome = *start.stopOutcome
			run.outcome.FinalState = info.State
			run.outcome.Verified = run.outcome.Verified && info.State == StateStopped
		}
	})
	return true
}

// settleCanceledSessionStart retries terminal persistence without reacquiring an already verified process.
func (m *Manager) settleCanceledSessionStart(ctx context.Context, accepted *acceptedSessionStart) error {
	session := accepted.session
	proc := accepted.proc
	if proc == nil || session.hasVerifiedStopProcess(proc) {
		proc = session.processHandle()
	}
	if accepted.run.stopOutcome != nil && accepted.run.stopOutcome.Verified && session.hasVerifiedStopProcess(proc) {
		return m.finalizeCanceledSessionStart(ctx, accepted)
	}
	if err := session.retainStoppingProcess(proc, m.now()); err != nil {
		return err
	}
	persistErr := m.persistSessionLifecycleState(ctx, session, false)
	cause, _ := session.stopCauseDetail()
	result, err := m.runTerminationLadder(ctx, proc, terminationTarget{
		beforeAction: func(ctx context.Context, phase StopPhase, elapsed time.Duration) error {
			recorded := &accepted.run.stopForcedRecorded
			if phase == StopPhaseKilled {
				recorded = &accepted.run.stopKilledRecorded
			}
			return recordStartStopObservation(recorded, func() error {
				return m.recordSessionStopEscalation(ctx, session, phase, elapsed, cause)
			})
		},
	})
	result.Cause = cause
	accepted.run.stopOutcome = &result.StopOutcome
	err = errors.Join(persistErr, err, result.observationErr)
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	if !result.Verified {
		observationErr := recordStartStopObservation(&accepted.run.stopVerificationRecorded, func() error {
			return m.recordSessionStopVerificationFailure(settleCtx, session, result.StopOutcome)
		})
		return errors.Join(err, observationErr)
	}
	result.StopOutcome = session.retainVerifiedStopOutcome(proc, result.StopOutcome)
	if accepted.proc == proc {
		accepted.proc = nil
	}
	return errors.Join(err, m.finalizeCanceledSessionStart(settleCtx, accepted))
}

// canceledStartWaitErr reports the launch error a canceled start settles with. A
// cancellation the stop request caused is the requested stop, classified by its
// cause like any other stop; recording it as a process failure would mark the
// runtime dead and refuse every later resume.
func canceledStartWaitErr(runCtx context.Context) error {
	cause := context.Cause(runCtx)
	if errors.Is(cause, errStartupCanceledByStop) {
		return nil
	}
	return cause
}

// waitSessionStopRun fences cached receipts against a newer startup cleanup owner.
func (m *Manager) waitSessionStopRun(ctx context.Context, id string, run *sessionStopRun) (StopOutcome, error) {
	outcome, err := waitSessionStopRun(ctx, run)
	if !signalClosed(run.done) || !outcome.Verified {
		return outcome, err
	}
	if start := m.sessionStartRun(id); start != nil && !start.activationCommitted.Load() {
		if startErr := waitForSessionStartRun(ctx, start); startErr != nil {
			return StopOutcome{}, errors.Join(err, startErr)
		}
	}
	return outcome, err
}

// canceledSessionStartPending joins the terminal owner even after its in-memory state is Stopped.
func (m *Manager) canceledSessionStartPending(accepted *acceptedSessionStart) bool {
	current, tracked := m.Get(accepted.session.ID)
	return accepted.recorderClosing || (tracked && current == accepted.session)
}

// settleAndTrackCanceledSessionStart keeps every cancellation entry point on the same retained owner.
func (m *Manager) settleAndTrackCanceledSessionStart(ctx context.Context, accepted *acceptedSessionStart) error {
	err := m.settleCanceledSessionStart(ctx, accepted)
	if m.canceledSessionStartPending(accepted) {
		return m.deferFailedStartCleanup(accepted, err)
	}
	return err
}

// recordStartStopObservation coalesces in-flight observations without holding a lock across I/O.
func recordStartStopObservation(recorded *atomic.Bool, record func() error) error {
	if !recorded.CompareAndSwap(false, true) {
		return nil
	}
	err := record()
	if err != nil {
		recorded.Store(false)
	}
	return err
}

// finalizeCanceledSessionStart keeps captured recorder cleanup owned after terminal projections finish.
func (m *Manager) finalizeCanceledSessionStart(ctx context.Context, accepted *acceptedSessionStart) error {
	err := m.finalizeStopped(ctx, accepted.session, canceledStartWaitErr(accepted.run.ctx))
	if current, tracked := m.Get(accepted.session.ID); tracked && current == accepted.session {
		return err
	}
	closeErr := m.cleanupFailedStart("", accepted.storage.recorder, nil)
	complete := closeErr == nil
	if cleanup, ok := accepted.storage.recorder.(interface{ CleanupComplete() bool }); ok {
		complete = cleanup.CleanupComplete()
	}
	accepted.recorderClosing = !complete
	if complete {
		accepted.storage.recorder = nil
	}
	return errors.Join(err, closeErr)
}
