//go:build test

package vectordb

import "context"

// ResetShared clears all cached stores using the provided context. Intended for tests.
func ResetShared(ctx context.Context) {
	defaultManager.reset(ctx)
}

func (m *Manager) reset(ctx context.Context) {
	m.mu.Lock()
	stores := make([]Store, 0, len(m.stores))
	for id, entry := range m.stores {
		stores = append(stores, entry.store)
		delete(m.stores, id)
	}
	m.mu.Unlock()
	for _, store := range stores {
		if store == nil {
			continue
		}
		_ = store.Close(ctx)
	}
}
