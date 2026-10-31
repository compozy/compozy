package session

import (
	"context"
	"errors"
	"time"
)

func (m *Manager) prepareStartingStopRun(
	ctx context.Context,
	run *sessionStopRun,
	cause StopCause,
	detail string,
) bool {
	session, start, handled, err := m.prepareStartingSessionStop(ctx, run.session.ID, cause, detail)
	if !handled {
		return false
	}
	run.prepareErr, run.err = err, err
	close(run.ready)
	if err != nil {
		close(run.done)
		return true
	}
	m.startTrackedPromptTask(func() {
		defer close(run.done)
		started := m.now()
		run.err = m.finishStartingSessionStop(context.WithoutCancel(ctx), session, start)
		info := session.Info()
		run.outcome = StopOutcome{
			FinalState: info.State, Verified: info.State == StateStopped,
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

func (m *Manager) settleCanceledSessionStart(ctx context.Context, accepted *acceptedSessionStart) error {
	session := accepted.session
	if err := session.retainStoppingProcess(accepted.proc, m.now()); err != nil {
		return err
	}
	persistErr := m.persistSessionLifecycleState(ctx, session, false)
	cause, _ := session.stopCauseDetail()
	result, err := m.runTerminationLadder(ctx, accepted.proc, terminationTarget{
		beforeAction: func(ctx context.Context, phase StopPhase, elapsed time.Duration) error {
			return m.recordSessionStopEscalation(ctx, session, phase, elapsed, cause)
		},
	})
	result.Cause = cause
	accepted.run.stopOutcome = &result.StopOutcome
	err = errors.Join(persistErr, err, result.observationErr)
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	if !result.Verified {
		return errors.Join(err, m.recordSessionStopVerificationFailure(settleCtx, session, result.StopOutcome))
	}
	result.StopOutcome = session.retainVerifiedStopOutcome(result.StopOutcome)
	return errors.Join(err, m.finalizeStopped(settleCtx, session, canceledStartWaitErr(accepted.run.ctx)))
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
