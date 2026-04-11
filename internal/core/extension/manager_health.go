package extensions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type healthCheckResponse struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

func (m *Manager) runHealthLoop(session *extensionSession) {
	defer m.backgroundGroup.Done()

	if session == nil || session.healthInterval <= 0 {
		return
	}

	ticker := time.NewTicker(session.healthInterval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-m.backgroundContext().Done():
			return
		case <-session.done:
			return
		case <-ticker.C:
		}

		healthy, err := m.runHealthCheck(session)
		if err != nil {
			failures++
			slog.Warn(
				"extension health probe failed",
				"component", "extension.manager",
				"extension", session.runtime.normalizedName(),
				"failures", failures,
				"err", err,
			)
			if failures >= 2 {
				m.markUnhealthy(session, fmt.Errorf("health probe threshold exceeded: %w", err))
				return
			}
			continue
		}

		failures = 0
		if healthy {
			continue
		}

		m.markUnhealthy(session, errors.New("extension reported unhealthy"))
		return
	}
}

func (m *Manager) runHealthCheck(session *extensionSession) (bool, error) {
	callCtx, cancel := context.WithTimeout(context.Background(), extensionHealthTimeout)
	defer cancel()

	var response healthCheckResponse
	if err := session.Call(callCtx, "health_check", struct{}{}, &response); err != nil {
		return false, err
	}
	return response.Healthy, nil
}

func (m *Manager) markUnhealthy(session *extensionSession, err error) {
	if session == nil || session.runtime == nil {
		return
	}

	session.degraded.Store(true)
	session.runtime.SetState(ExtensionStateDegraded)
	session.failureOnce.Do(func() {
		if emitErr := m.emitFailureEvent(
			context.Background(),
			session.runtime,
			extensionFailurePhaseHealth,
			err,
		); emitErr != nil {
			slog.Warn(
				"emit extension health failure event",
				"component", "extension.manager",
				"extension", session.runtime.normalizedName(),
				"err", emitErr,
			)
		}
	})

	go func() {
		if shutdownErr := m.shutdownSession(
			context.Background(),
			session,
			shutdownReasonHealthFailed,
		); shutdownErr != nil {
			slog.Warn(
				"shutdown unhealthy extension",
				"component", "extension.manager",
				"extension", session.runtime.normalizedName(),
				"err", shutdownErr,
			)
		}
	}()
}
