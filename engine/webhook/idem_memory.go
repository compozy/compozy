package webhook

import (
	"context"
	"sync"
	"time"
)

// memSvc provides an in-memory idempotency store with TTL-based eviction.
// It is suitable for standalone/dev modes only. Not safe for distributed use.
type memSvc struct {
	mu   sync.Mutex
	data map[string]time.Time // key -> expiry time
}

// NewInMemoryService creates a new in-memory idempotency Service implementation.
func NewInMemoryService() Service {
	return &memSvc{data: make(map[string]time.Time)}
}

// CheckAndSet checks whether the key already exists and, if not, sets it with TTL.
func (m *memSvc) CheckAndSet(_ context.Context, key string, ttl time.Duration) error {
	now := time.Now()
	exp := now.Add(ttl)
	m.mu.Lock()
	// Lazy sweep: delete expired key if present
	if cur, ok := m.data[key]; ok {
		if cur.After(now) {
			m.mu.Unlock()
			return ErrDuplicate
		}
		delete(m.data, key)
	}
	m.data[key] = exp
	m.mu.Unlock()
	return nil
}
