//go:build test

package vectordb

import "context"

// ResetShared clears all cached stores using the provided context. Intended for tests.
func ResetShared(ctx context.Context) {
	defaultManager.reset(ctx)
}

func (m *Manager) reset(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, entry := range m.stores {
		_ = entry.store.Close(ctx)
		delete(m.stores, id)
	}
}
