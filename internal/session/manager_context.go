package session

import "context"

func (m *Manager) lifecycleCleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(m.lifecycleCtx), defaultLifecycleTimeout)
}
