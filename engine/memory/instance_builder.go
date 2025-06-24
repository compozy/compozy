package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/compozy/compozy/pkg/logger"
)

// memoryComponents holds all the dependencies needed for a memory instance
type memoryComponents struct {
	store            memcore.Store
	lockManager      *instance.LockManagerImpl
	tokenManager     *TokenMemoryManager
	flushingStrategy *HybridFlushingStrategy
}

// buildMemoryComponents creates all the necessary components for a memory instance
func (mm *Manager) buildMemoryComponents(
	resourceCfg *memcore.Resource,
	projectIDVal string,
) (*memoryComponents, error) {
	redisStore := store.NewRedisMemoryStore(mm.baseRedisClient, "")
	lockManager, err := mm.createLockManager(projectIDVal, resourceCfg)
	if err != nil {
		return nil, err
	}
	tokenManager, err := mm.createTokenManager(resourceCfg)
	if err != nil {
		return nil, err
	}
	flushingStrategy, err := mm.createFlushingStrategy(resourceCfg, tokenManager)
	if err != nil {
		return nil, err
	}
	return &memoryComponents{
		store:            redisStore,
		lockManager:      lockManager,
		tokenManager:     tokenManager,
		flushingStrategy: flushingStrategy,
	}, nil
}

// createLockManager creates a memory lock manager with project namespacing and TTL configuration
//
//nolint:unparam // error return is intentional for future extensibility and consistency
func (mm *Manager) createLockManager(
	projectIDVal string,
	resourceCfg *memcore.Resource,
) (*instance.LockManagerImpl, error) {
	// Create distributed lock adapter using existing Redis LockManager
	locker := newLockManagerAdapter(mm.baseLockManager, mm.log, projectIDVal)
	lockManager := instance.NewLockManager(locker)

	// Configure TTLs from resource configuration if available
	if resourceCfg != nil {
		// Parse TTL durations from resource configuration
		if resourceCfg.AppendTTL != "" {
			if ttl, err := time.ParseDuration(resourceCfg.AppendTTL); err == nil {
				lockManager = lockManager.WithAppendTTL(ttl)
			} else {
				mm.log.Warn("Invalid append TTL format, using default",
					"resource_id", resourceCfg.ID,
					"append_ttl", resourceCfg.AppendTTL,
					"error", err)
			}
		}

		if resourceCfg.ClearTTL != "" {
			if ttl, err := time.ParseDuration(resourceCfg.ClearTTL); err == nil {
				lockManager = lockManager.WithClearTTL(ttl)
			} else {
				mm.log.Warn("Invalid clear TTL format, using default",
					"resource_id", resourceCfg.ID,
					"clear_ttl", resourceCfg.ClearTTL,
					"error", err)
			}
		}

		if resourceCfg.FlushTTL != "" {
			if ttl, err := time.ParseDuration(resourceCfg.FlushTTL); err == nil {
				lockManager = lockManager.WithFlushTTL(ttl)
			} else {
				mm.log.Warn("Invalid flush TTL format, using default",
					"resource_id", resourceCfg.ID,
					"flush_ttl", resourceCfg.FlushTTL,
					"error", err)
			}
		}
	}

	return lockManager, nil
}

// lockManagerAdapter adapts cache.LockManager to instance.Locker with proper distributed locking
type lockManagerAdapter struct {
	manager   cache.LockManager
	log       logger.Logger
	projectID string
}

// newLockManagerAdapter creates a new lock manager adapter with proper configuration
func newLockManagerAdapter(manager cache.LockManager, log logger.Logger, projectID string) instance.Locker {
	return &lockManagerAdapter{
		manager:   manager,
		log:       log,
		projectID: projectID,
	}
}

// Lock implements instance.Locker using Redis distributed locking with retry logic
func (lma *lockManagerAdapter) Lock(ctx context.Context, key string, ttl time.Duration) (instance.Lock, error) {
	lockKey := fmt.Sprintf("memory:%s:%s", lma.projectID, key)
	cacheLock, err := lma.acquireWithRetry(ctx, lockKey, ttl)
	if err != nil {
		return nil, err
	}
	return lma.wrapLock(lockKey, cacheLock), nil
}

// acquireWithRetry implements retry logic for lock acquisition with exponential backoff
func (lma *lockManagerAdapter) acquireWithRetry(
	ctx context.Context,
	lockKey string,
	ttl time.Duration,
) (cache.Lock, error) {
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		cacheLock, err := lma.tryAcquire(ctx, lockKey, ttl)
		if err == nil {
			lma.log.Debug("Successfully acquired distributed lock",
				"key", lockKey, "ttl", ttl, "attempt", attempt+1)
			return cacheLock, nil
		}
		lastErr = err
		if !lma.shouldRetry(ctx, attempt, maxRetries) {
			break
		}
		if retryErr := lma.waitForRetry(ctx, attempt, lockKey, maxRetries, err); retryErr != nil {
			return nil, retryErr
		}
	}
	return nil, lma.formatAcquisitionError(lockKey, maxRetries, lastErr)
}

// tryAcquire attempts a single lock acquisition with timeout
func (lma *lockManagerAdapter) tryAcquire(ctx context.Context, lockKey string, ttl time.Duration) (cache.Lock, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return lma.manager.Acquire(attemptCtx, lockKey, ttl)
}

// shouldRetry determines if we should retry lock acquisition
func (lma *lockManagerAdapter) shouldRetry(ctx context.Context, attempt, maxRetries int) bool {
	return attempt < maxRetries && ctx.Err() == nil
}

// waitForRetry handles the exponential backoff delay between retry attempts
func (lma *lockManagerAdapter) waitForRetry(
	ctx context.Context,
	attempt int,
	lockKey string,
	maxRetries int,
	err error,
) error {
	const baseDelay = 50 * time.Millisecond
	// Safe exponential backoff with bounds checking to prevent overflow
	if attempt < 0 || attempt > 10 {
		attempt = 10 // Cap at reasonable maximum
	}
	delay := time.Duration(1<<attempt) * baseDelay
	lma.log.Debug("Lock acquisition failed, retrying",
		"key", lockKey, "attempt", attempt+1, "max_retries", maxRetries, "delay", delay, "error", err)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// formatAcquisitionError creates a formatted error for failed lock acquisition
func (lma *lockManagerAdapter) formatAcquisitionError(lockKey string, maxRetries int, lastErr error) error {
	lma.log.Error("Failed to acquire distributed lock after retries",
		"key", lockKey, "max_retries", maxRetries, "error", lastErr)
	return fmt.Errorf("failed to acquire distributed lock for key %s after %d attempts: %w",
		lockKey, maxRetries+1, lastErr)
}

// wrapLock wraps a cache.Lock to implement instance.Lock interface
func (lma *lockManagerAdapter) wrapLock(lockKey string, cacheLock cache.Lock) instance.Lock {
	return &distributedLock{
		key:       lockKey,
		cacheLock: cacheLock,
		adapter:   lma,
	}
}

// distributedLock implements instance.Lock using Redis distributed locking
type distributedLock struct {
	key       string
	cacheLock cache.Lock
	adapter   *lockManagerAdapter
}

// Unlock implements instance.Lock by releasing the Redis distributed lock
func (dl *distributedLock) Unlock(ctx context.Context) error {
	if dl.cacheLock == nil {
		return fmt.Errorf("lock already released or never acquired")
	}

	err := dl.cacheLock.Release(ctx)
	if err != nil {
		dl.adapter.log.Error("Failed to release distributed lock",
			"key", dl.key,
			"error", err)
		return fmt.Errorf("failed to release distributed lock for key %s: %w", dl.key, err)
	}

	dl.adapter.log.Debug("Successfully released distributed lock",
		"key", dl.key)

	// Clear the lock reference to prevent double release
	dl.cacheLock = nil
	return nil
}

// createTokenManager creates a token manager for the given resource configuration
func (mm *Manager) createTokenManager(resourceCfg *memcore.Resource) (*TokenMemoryManager, error) {
	model := DefaultTokenCounterModel
	tokenCounter, err := mm.getOrCreateTokenCounter(model)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create token counter for resource '%s' with model '%s': %w",
			resourceCfg.ID,
			model,
			err,
		)
	}
	tokenManager, err := NewTokenMemoryManager(resourceCfg, tokenCounter, mm.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create token manager for resource '%s': %w", resourceCfg.ID, err)
	}
	return tokenManager, nil
}

// createFlushingStrategy creates a flushing strategy for the given resource configuration
func (mm *Manager) createFlushingStrategy(
	resourceCfg *memcore.Resource,
	tokenManager *TokenMemoryManager,
) (*HybridFlushingStrategy, error) {
	var summarizer MessageSummarizer
	if resourceCfg.FlushingStrategy != nil && resourceCfg.FlushingStrategy.Type == memcore.HybridSummaryFlushing {
		tokenCounter, err := mm.getOrCreateTokenCounter(DefaultTokenCounterModel)
		if err != nil {
			return nil, fmt.Errorf("failed to create token counter for summarizer: %w", err)
		}
		summarizer = NewRuleBasedSummarizer(tokenCounter, 1, 1)
	}
	flushingStrategy, err := NewHybridFlushingStrategy(resourceCfg.FlushingStrategy, summarizer, tokenManager)
	if err != nil {
		if resourceCfg.FlushingStrategy == nil {
			return mm.createDefaultFlushingStrategy(resourceCfg, tokenManager)
		}
		return nil, fmt.Errorf("failed to create flushing strategy for resource '%s': %w", resourceCfg.ID, err)
	}
	return flushingStrategy, nil
}

// createDefaultFlushingStrategy creates a default FIFO flushing strategy when none is configured
func (mm *Manager) createDefaultFlushingStrategy(
	resourceCfg *memcore.Resource,
	tokenManager *TokenMemoryManager,
) (*HybridFlushingStrategy, error) {
	defaultFlushCfg := &memcore.FlushingStrategyConfig{Type: memcore.SimpleFIFOFlushing}
	flushingStrategy, err := NewHybridFlushingStrategy(defaultFlushCfg, nil, tokenManager)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create default flushing strategy for resource '%s': %w",
			resourceCfg.ID,
			err,
		)
	}
	return flushingStrategy, nil
}

// createMemoryInstance creates the final memory instance with all components
func (mm *Manager) createMemoryInstance(
	sanitizedKey, projectIDVal string,
	resourceCfg *memcore.Resource,
	components *memoryComponents,
) (memcore.Memory, error) {
	// Build token counter for this instance
	tokenCounter, err := mm.getOrCreateTokenCounter(resourceCfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create token counter: %w", err)
	}
	// Use the instance builder
	instanceBuilder := instance.NewBuilder().
		WithInstanceID(sanitizedKey).
		WithResourceID(resourceCfg.ID).
		WithProjectID(projectIDVal).
		WithResourceConfig(resourceCfg).
		WithStore(components.store).
		WithLockManager(components.lockManager).
		WithTokenCounter(tokenCounter).
		WithFlushingStrategy(components.flushingStrategy).
		WithTemporalClient(mm.temporalClient).
		WithTemporalTaskQueue(mm.temporalTaskQueue).
		WithPrivacyManager(mm.privacyManager).
		WithLogger(mm.log)
	if err := instanceBuilder.Validate(); err != nil {
		return nil, fmt.Errorf("instance builder validation failed: %w", err)
	}
	memInstance, err := instanceBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build memory instance: %w", err)
	}
	return memInstance, nil
}
