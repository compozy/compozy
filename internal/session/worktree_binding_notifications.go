package session

import "context"

// WorktreeBindingNotifier refreshes daemon projections after durable binding acceptance.
type WorktreeBindingNotifier func(context.Context, *Info, string)

func (m *Manager) AddWorktreeBindingNotifier(notify WorktreeBindingNotifier) {
	if notify == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.worktreeBindingNotifiers = append(m.worktreeBindingNotifiers, notify)
}

func (m *Manager) notifyWorktreeBindingChanged(ctx context.Context, info *Info, previous string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	m.mu.RLock()
	callbacks := append([]WorktreeBindingNotifier(nil), m.worktreeBindingNotifiers...)
	m.mu.RUnlock()
	for _, notify := range callbacks {
		notify(ctx, info, previous)
	}
	current, err := m.Status(ctx, info.ID)
	if err != nil || current == nil {
		current = info
	}
	event := sessionCatalogEventFromInfo(CatalogEventUpserted, current)
	event.ExecutionLocationChanged = true
	m.publishSessionCatalogEvent(event)
}
