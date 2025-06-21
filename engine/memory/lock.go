package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/CompoZy/llm-router/engine/infra/cache" // Assuming this is the correct path for cache.LockManager
)

// MemoryLockManager is a wrapper around the cache.LockManager to provide
// context or specific behaviors for memory-related locks.
// For Task 1.0, this can be a thin wrapper, or we might find direct usage of
// cache.LockManager is sufficient. This wrapper provides a dedicated type
// and a place for memory-specific locking logic if it evolves.
type MemoryLockManager struct {
	internalLockManager cache.LockManager
	lockKeyPrefix       string
}

// NewMemoryLockManager creates a new manager for memory locks.
// It takes an existing cache.LockManager instance.
func NewMemoryLockManager(manager cache.LockManager, lockKeyPrefix string) (*MemoryLockManager, error) {
	if manager == nil {
		return nil, fmt.Errorf("internal lock manager cannot be nil")
	}
	if lockKeyPrefix == "" {
		lockKeyPrefix = "mlock:" // memory lock prefix
	}
	return &MemoryLockManager{
		internalLockManager: manager,
		lockKeyPrefix:       lockKeyPrefix,
	}, nil
}

// Acquire attempts to acquire a distributed lock for a memory resource.
// 'resourceID' could be the memory instance key.
// 'ttl' is the time-to-live for the lock.
func (m *MemoryLockManager) Acquire(ctx context.Context, resourceID string, ttl time.Duration) (cache.Lock, error) {
	lockKey := m.prefixedResourceID(resourceID)
	return m.internalLockManager.Acquire(ctx, lockKey, ttl)
}

// prefixedResourceID generates the actual key used in Redis for the lock.
func (m *MemoryLockManager) prefixedResourceID(resourceID string) string {
	return m.lockKeyPrefix + resourceID
}

// GetMetrics allows passthrough to the underlying LockManager's metrics if available.
// This depends on the actual cache.LockManager implementation.
// For now, this is a placeholder; specific metric access would need casting
// or the cache.LockManager interface would need to expose metrics.
func (m *MemoryLockManager) GetMetrics() (interface{}, error) {
	if metricsProvider, ok := m.internalLockManager.(interface{ GetMetrics() cache.LockMetrics }); ok {
		return metricsProvider.GetMetrics(), nil
	}
	return nil, fmt.Errorf("underlying lock manager does not provide GetMetrics method")
}
