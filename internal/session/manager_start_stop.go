package session

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
)

func (m *Manager) stopStartingSession(ctx context.Context, id string, cause StopCause, detail string) (bool, error) {
	session, run, handled, err := m.prepareStartingSessionStop(ctx, id, cause, detail)
	if !handled || err != nil {
		return handled, err
	}
	return true, m.finishStartingSessionStop(ctx, session, run)
}

func (m *Manager) finishStartingSessionStop(ctx context.Context, session *Session, run *sessionStartRun) error {
	if run == nil {
		return m.finalizeStopped(ctx, session, nil)
	}
	started := m.now()
	waitCtx, cancel := context.WithTimeout(ctx, m.stopConfig.CooperativeGrace+stopForcedGrace+stopKillGrace)
	defer cancel()
	err := waitForSessionStartRun(waitCtx, run)
	if signalClosed(run.done) || session.Info().State == StateStopped {
		return err
	}
	outcome := StopOutcome{FinalState: StateStopping, Phase: StopPhaseCooperative, Elapsed: m.now().Sub(started)}
	outcome.Cause, _ = session.stopCauseDetail()
	diagnosticCtx, cancelDiagnostic := m.lifecycleCleanupContext()
	defer cancelDiagnostic()
	return errors.Join(
		ErrStopVerificationFailed,
		err,
		m.recordSessionStopVerificationFailure(diagnosticCtx, session, outcome),
	)
}

func (m *Manager) prepareStartingSessionStop(
	ctx context.Context,
	id string,
	cause StopCause,
	detail string,
) (*Session, *sessionStartRun, bool, error) {
	session, run := m.startingStopTarget(id)
	if session == nil {
		return nil, nil, false, nil
	}
	var releaseCommit func()
	releaseLaunchCommit := func() {
		if releaseCommit == nil {
			return
		}
		releaseCommit()
		releaseCommit = nil
	}
	defer releaseLaunchCommit()
	if run != nil {
		if err := waitForSessionStartRecorder(ctx, run); err != nil {
			return session, run, true, fmt.Errorf("session: wait for starting recorder for %q: %w", id, err)
		}
		var err error
		releaseCommit, err = acquireSessionStartLaunchCommit(ctx, run)
		if err != nil {
			return session, run, true, fmt.Errorf("session: acquire starting launch commit for %q: %w", id, err)
		}
		state := session.Info().State
		if state != StateStarting && state != StateStopping {
			releaseLaunchCommit()
			return nil, nil, false, nil
		}
	}

	writeMeta, _, err := session.prepareStop(m.now(), cause, detail)
	if err != nil {
		releaseLaunchCommit()
		return session, run, true, fmt.Errorf("session: prepare starting stop for %q: %w", id, err)
	}
	if writeMeta {
		if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
			persistErr := fmt.Errorf("session: persist starting stop for %q: %w", id, err)
			if run != nil {
				run.cancel(persistErr)
			}
			releaseLaunchCommit()
			return session, run, true, persistErr
		}
	}
	if run == nil {
		return session, nil, true, nil
	}
	cancelCause := fmt.Errorf("session: startup canceled for %q", strings.TrimSpace(id))
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		cancelCause = fmt.Errorf("session: startup canceled for %q: %s", strings.TrimSpace(id), trimmed)
	}
	run.cancel(cancelCause)
	releaseLaunchCommit()
	return session, run, true, nil
}

func (m *Manager) startingStopTarget(id string) (*Session, *sessionStartRun) {
	session, ok := m.Get(id)
	if !ok || session == nil {
		return nil, nil
	}
	state := session.Info().State
	if state != StateStarting && state != StateStopping {
		return nil, nil
	}
	run := m.sessionStartRun(id)
	if run == nil && session.processHandle() != nil {
		return nil, nil
	}
	return session, run
}

func (m *Manager) shutdownSessionStarts(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session: shutdown starts context is required")
	}
	m.startLifecycle.mu.Lock()
	m.startLifecycle.closing = true
	runs := make(map[string]*sessionStartRun, len(m.startLifecycle.runs))
	maps.Copy(runs, m.startLifecycle.runs)
	m.startLifecycle.mu.Unlock()

	var shutdownErr error
	for id, run := range runs {
		session, ok := m.Get(id)
		if ok && session != nil {
			state := session.Info().State
			if state == StateStarting || state == StateStopping {
				_, stopErr := m.stopStartingSession(ctx, id, CauseShutdown, "manager shutdown")
				shutdownErr = errors.Join(shutdownErr, stopErr)
				continue
			}
		}
		if waitErr := waitForSessionStartRun(ctx, run); waitErr != nil {
			shutdownErr = errors.Join(shutdownErr, waitErr)
		}
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	m.startLifecycle.wg.Wait()
	return nil
}
