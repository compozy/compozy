package session

import "context"

type instanceMainGuard interface {
	GuardInstanceMain(context.Context, string) error
}

func (m *Manager) guardInstanceMain(ctx context.Context, sessionID string) error {
	if guard, ok := m.sessionCatalog.(instanceMainGuard); ok {
		return guard.GuardInstanceMain(ctx, sessionID)
	}
	return nil
}
