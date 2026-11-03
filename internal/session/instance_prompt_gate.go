package session

import (
	"context"
	"errors"
	"strings"
)

type instanceManagedRunKey struct{}

// WithManagedRun marks a daemon-owned Goal/Loop prompt; it is not a public caller override.
func WithManagedRun(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, instanceManagedRunKey{}, strings.TrimSpace(runID))
}

func (m *Manager) admitManagedInstancePrompt(ctx context.Context, sessionID string) error {
	runID, ok := ctx.Value(instanceManagedRunKey{}).(string)
	if !ok || runID == "" {
		return nil
	}
	guard, ok := m.promptAdmissionStore.(interface {
		RequireInstanceSessionActive(context.Context, string) error
	})
	if !ok {
		return errors.New("session: managed prompt instance guard unavailable")
	}
	return guard.RequireInstanceSessionActive(ctx, sessionID)
}

// ReleaseInstanceInputs rebinds a stopped session when necessary and nudges its durable selector.
// Later remains an independent operator hold through lookupOrResumePromptSession.
func (m *Manager) ReleaseInstanceInputs(ctx context.Context, sessionID string) error {
	if _, err := m.lookupOrResumePromptSession(ctx, sessionID, false); err != nil {
		return err
	}
	m.startNextQueuedInputPrompt(sessionID)
	return nil
}
