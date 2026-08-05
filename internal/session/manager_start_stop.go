package session

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
)

func (m *Manager) stopStartingSession(
	ctx context.Context,
	id string,
	cause StopCause,
	detail string,
) (bool, error) {
	session, ok := m.Get(id)
	if !ok || session == nil {
		return false, nil
	}
	state := session.Info().State
	if state != StateStarting && state != StateStopping {
		return false, nil
	}

	run := m.sessionStartRun(id)
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
			return true, fmt.Errorf("session: wait for starting recorder for %q: %w", id, err)
		}
		var err error
		releaseCommit, err = acquireSessionStartLaunchCommit(ctx, run)
		if err != nil {
			return true, fmt.Errorf("session: acquire starting launch commit for %q: %w", id, err)
		}
		state = session.Info().State
		if state != StateStarting && state != StateStopping {
			releaseLaunchCommit()
			return false, nil
		}
	}

	writeMeta, _, err := session.prepareStop(m.now(), cause, detail)
	if err != nil {
		releaseLaunchCommit()
		return true, fmt.Errorf("session: prepare starting stop for %q: %w", id, err)
	}
	if writeMeta {
		if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
			persistErr := fmt.Errorf("session: persist starting stop for %q: %w", id, err)
			if run != nil {
				run.cancel(persistErr)
			}
			releaseLaunchCommit()
			if run != nil {
				return true, errors.Join(persistErr, waitForSessionStartRun(ctx, run))
			}
			return true, persistErr
		}
	}
	if run == nil {
		return true, m.finalizeStopped(ctx, session, nil)
	}
	cancelCause := fmt.Errorf("session: startup canceled for %q", strings.TrimSpace(id))
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		cancelCause = fmt.Errorf("session: startup canceled for %q: %s", strings.TrimSpace(id), trimmed)
	}
	run.cancel(cancelCause)
	releaseLaunchCommit()
	if err := waitForSessionStartRun(ctx, run); err != nil && !errors.Is(err, context.Canceled) {
		return true, err
	}
	return true, nil
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
