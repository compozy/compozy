package session

import (
	"context"

	"github.com/compozy/compozy/internal/config"
)

// SessionEnvironment returns only references from the bound runtime, never secret values.
func (m *Manager) SessionEnvironment(ctx context.Context, identity PromptRunIdentity) (config.SessionEnv, error) {
	active, err := m.ActivePromptRun(ctx, identity.SessionID)
	if err != nil {
		return nil, err
	}
	if active.WorkspaceID != identity.WorkspaceID || active.ProfileID != identity.ProfileID ||
		active.RunID != identity.RunID || active.Generation != identity.Generation {
		return nil, ErrPromptNotActive
	}
	session, ok := m.Get(identity.SessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.WorkspaceID != identity.WorkspaceID || session.ProfileID != identity.ProfileID ||
		session.RuntimeGeneration != identity.Generation {
		return nil, ErrPromptNotActive
	}
	return config.MergeSessionEnv(nil, session.providerRoute.SessionEnv), nil
}
