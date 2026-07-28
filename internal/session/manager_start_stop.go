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

	writeMeta, _, err := session.prepareStop(m.now(), cause, detail)
	if err != nil {
		return true, fmt.Errorf("session: prepare starting stop for %q: %w", id, err)
	}
	if writeMeta {
		if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
			return true, fmt.Errorf("session: persist starting stop for %q: %w", id, err)
		}
	}
	run := m.sessionStartRun(id)
	if run == nil {
		return true, m.finalizeStopped(ctx, session, nil)
	}
	cancelCause := fmt.Errorf("session: startup canceled for %q", strings.TrimSpace(id))
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		cancelCause = fmt.Errorf("session: startup canceled for %q: %s", strings.TrimSpace(id), trimmed)
	}
	run.cancel(cancelCause)
	if err := waitForSessionStartRun(ctx, run); err != nil && !errors.Is(err, context.Canceled) {
		return true, err
	}
	return true, nil
}

func (m *Manager) shutdownSessionStarts(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session: shutdown starts context is required")
	}
	m.startMu.Lock()
	m.startClosing = true
	runs := make(map[string]*sessionStartRun, len(m.startRuns))
	maps.Copy(runs, m.startRuns)
	m.startMu.Unlock()

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
	m.startWG.Wait()
	return nil
}
