package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// CreateAccepted validates and durably registers a starting session before
// launching provider startup under the manager lifecycle context.
func (m *Manager) CreateAccepted(ctx context.Context, opts CreateAcceptedOpts) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: create accepted context is required")
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	spec, err := m.prepareCreateStart(ctx, opts.Session)
	if err != nil {
		return nil, err
	}
	accepted, err := m.acceptSessionStart(ctx, m.lifecycleCtx, &spec)
	if err != nil {
		return nil, err
	}
	if err := m.stageAcceptedInitialPrompt(ctx, accepted.session.ID, opts.InitialPrompt); err != nil {
		return nil, m.discardAcceptedCreate(accepted, err)
	}
	accepted.async = true
	accepted.persistFailure = true
	info := accepted.session.Info()
	go func() {
		if runErr := m.runAcceptedSessionStartAndDispatch(accepted); runErr != nil {
			m.sessionLogger(accepted.session).Warn(
				"session.start.background_failed",
				"error", runErr,
			)
		}
	}()
	return info, nil
}

func (m *Manager) stageAcceptedInitialPrompt(ctx context.Context, sessionID string, prompt string) error {
	message := strings.TrimSpace(prompt)
	if message == "" {
		return nil
	}
	if m.inputQueue == nil {
		return errors.New("session: input queue is not configured")
	}
	// Detach from request cancellation so a client disconnect after durable
	// acceptance cannot fail enqueue and roll the session back.
	stageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	generation, err := m.currentInputGeneration(stageCtx, sessionID)
	if err != nil {
		return fmt.Errorf("session: read initial prompt queue generation for %q: %w", sessionID, err)
	}
	if _, _, err := m.inputQueue.Enqueue(stageCtx, sessionID, message, generation); err != nil {
		return fmt.Errorf("session: stage initial prompt for %q: %w", sessionID, err)
	}
	return nil
}

func (m *Manager) discardAcceptedCreate(accepted *acceptedSessionStart, cause error) (err error) {
	if accepted == nil || accepted.run == nil {
		return errors.Join(cause, errors.New("session: accepted start is required for cleanup"))
	}
	accepted.run.cancel(cause)
	defer func() {
		m.finishSessionStartRun(accepted.spec.sessionID, accepted.run, err)
	}()
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(m.lifecycleCtx), defaultLifecycleTimeout)
	defer cancel()
	return m.discardAcceptedSessionStart(cleanupCtx, accepted, cause)
}
