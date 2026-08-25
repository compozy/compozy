package session

import (
	"context"
	"errors"
	"fmt"
)

// Shutdown releases session-manager background resources after active sessions stop.
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.stopDeletedSessionWindowReconciliation()
	var shutdownErr error
	if m.waitRegistry != nil {
		m.waitRegistry.close()
	}
	m.closeSessionResumes()
	if err := m.shutdownSessionStarts(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("session: shut down starts: %w", err))
	}
	if err := m.waitForSessionResumes(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("session: wait for resumes: %w", err))
	}
	if err := m.WaitForPromptDrains(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("session: wait for prompt drains during shutdown: %w", err))
	}
	if err := m.shutdownCompactions(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("session: shut down compactions: %w", err))
	}
	if err := m.shutdownProcessWatchers(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("session: shut down process watchers: %w", err))
	}
	if err := m.shutdownQueryStoreRuntime(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("session: shut down query store runtime: %w", err))
	}
	m.waitForDeletedSessionWindowReconciliation()
	return shutdownErr
}
