package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache" // Assuming this is the correct path for cache.LockManager
)

// LockManager is a wrapper around the cache.LockManager to provide
// context or specific behaviors for memory-related locks.
// For Task 1.0, this can be a thin wrapper, or we might find direct usage of
// cache.LockManager is sufficient. This wrapper provides a dedicated type
// and a place for memory-specific locking logic if it evolves.
type LockManager struct {
	internalLockManager cache.LockManager
	lockKeyPrefix       string
}

// memoryLock wraps a cache.Lock to provide the unprefixed resource ID
type memoryLock struct {
	cache.Lock
	originalResourceID string
}

// NewLockManager creates a new manager for memory locks.
// It takes an existing cache.LockManager instance.
func NewLockManager(manager cache.LockManager, lockKeyPrefix string) (*LockManager, error) {
	if manager == nil {
		return nil, fmt.Errorf("internal lock manager cannot be nil")
	}
	if lockKeyPrefix == "" {
		lockKeyPrefix = "mlock:" // memory lock prefix
	}
	return &LockManager{
		internalLockManager: manager,
		lockKeyPrefix:       lockKeyPrefix,
	}, nil
}

// Acquire attempts to acquire a distributed lock for a memory resource.
// 'resourceID' could be the memory instance key.
// 'ttl' is the time-to-live for the lock.
func (m *LockManager) Acquire(ctx context.Context, resourceID string, ttl time.Duration) (cache.Lock, error) {
	lockKey := m.prefixedResourceID(resourceID)
	lock, err := m.internalLockManager.Acquire(ctx, lockKey, ttl)
	if err != nil {
		return nil, err
	}
	return &memoryLock{Lock: lock, originalResourceID: resourceID}, nil
}

// prefixedResourceID generates the actual key used in Redis for the lock.
func (m *LockManager) prefixedResourceID(resourceID string) string {
	return m.lockKeyPrefix + resourceID
}

// Resource returns the original unprefixed resource ID
func (ml *memoryLock) Resource() string {
	return ml.originalResourceID
}

// GetMetrics allows passthrough to the underlying LockManager's metrics if available.
// This depends on the actual cache.LockManager implementation.
// For now, this is a placeholder; specific metric access would need casting
// or the cache.LockManager interface would need to expose metrics.
func (m *LockManager) GetMetrics() (*cache.LockMetrics, error) {
	switch mp := m.internalLockManager.(type) {
	case interface{ GetMetrics() cache.LockMetrics }:
		// Get the metrics value (returns by value)
		metricsValue := mp.GetMetrics()
		// Create a new LockMetrics without copying the mutex
		metrics := &cache.LockMetrics{
			AcquisitionsTotal:  metricsValue.AcquisitionsTotal,
			AcquisitionsFailed: metricsValue.AcquisitionsFailed,
			ReleasesTotal:      metricsValue.ReleasesTotal,
			ReleasesFailed:     metricsValue.ReleasesFailed,
			RefreshesTotal:     metricsValue.RefreshesTotal,
			RefreshesFailed:    metricsValue.RefreshesFailed,
			AcquisitionTime:    metricsValue.AcquisitionTime,
		}
		return metrics, nil
	case interface{ GetMetrics() *cache.LockMetrics }:
		// Get the metrics pointer (returns by pointer)
		metricsPtr := mp.GetMetrics()
		if metricsPtr == nil {
			return nil, fmt.Errorf("underlying lock manager returned nil metrics")
		}
		// Create a new LockMetrics without copying the mutex
		metrics := &cache.LockMetrics{
			AcquisitionsTotal:  metricsPtr.AcquisitionsTotal,
			AcquisitionsFailed: metricsPtr.AcquisitionsFailed,
			ReleasesTotal:      metricsPtr.ReleasesTotal,
			ReleasesFailed:     metricsPtr.ReleasesFailed,
			RefreshesTotal:     metricsPtr.RefreshesTotal,
			RefreshesFailed:    metricsPtr.RefreshesFailed,
			AcquisitionTime:    metricsPtr.AcquisitionTime,
		}
		return metrics, nil
	default:
		return nil, fmt.Errorf("underlying lock manager does not provide GetMetrics method")
	}
}
