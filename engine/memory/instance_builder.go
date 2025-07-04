package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance"
	"github.com/compozy/compozy/engine/memory/instance/eviction"
	"github.com/compozy/compozy/engine/memory/instance/strategies"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/compozy/compozy/pkg/logger"
)

// memoryComponents holds all the dependencies needed for a memory instance
type memoryComponents struct {
	store            memcore.Store
	lockManager      *instance.LockManagerImpl
	tokenManager     *TokenMemoryManager
	flushingStrategy memcore.FlushStrategy
	evictionPolicy   instance.EvictionPolicy
}

// buildMemoryComponents creates all the necessary components for a memory instance
func (mm *Manager) buildMemoryComponents(
	resourceCfg *memcore.Resource,
	projectIDVal string,
) (*memoryComponents, error) {
	// Build the key prefix with namespace: compozy:{project_id}:memory
	keyPrefix := fmt.Sprintf("compozy:%s:memory", projectIDVal)
	redisStore := store.NewRedisMemoryStore(mm.baseRedisClient, keyPrefix)
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
	evictionPolicy := mm.createEvictionPolicy(resourceCfg)
	return &memoryComponents{
		store:            redisStore,
		lockManager:      lockManager,
		tokenManager:     tokenManager,
		flushingStrategy: flushingStrategy,
		evictionPolicy:   evictionPolicy,
	}, nil
}

// createLockManager creates a memory lock manager with project namespacing and TTL configuration
//

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
			ttl, err := time.ParseDuration(resourceCfg.AppendTTL)
			if err != nil {
				return nil, fmt.Errorf("invalid append TTL format for resource %s: %q: %w",
					resourceCfg.ID, resourceCfg.AppendTTL, err)
			}
			lockManager = lockManager.WithAppendTTL(ttl)
		}

		if resourceCfg.ClearTTL != "" {
			ttl, err := time.ParseDuration(resourceCfg.ClearTTL)
			if err != nil {
				return nil, fmt.Errorf("invalid clear TTL format for resource %s: %q: %w",
					resourceCfg.ID, resourceCfg.ClearTTL, err)
			}
			lockManager = lockManager.WithClearTTL(ttl)
		}

		if resourceCfg.FlushTTL != "" {
			ttl, err := time.ParseDuration(resourceCfg.FlushTTL)
			if err != nil {
				return nil, fmt.Errorf("invalid flush TTL format for resource %s: %q: %w",
					resourceCfg.ID, resourceCfg.FlushTTL, err)
			}
			lockManager = lockManager.WithFlushTTL(ttl)
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
	// Check if this is a flush lock - flush locks should fail fast without retry
	isFlushLock := strings.Contains(lockKey, ":flush_lock")

	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		cacheLock, err := lma.tryAcquire(ctx, lockKey, ttl)
		if err == nil {
			lma.log.Debug("Successfully acquired distributed lock",
				"key", lockKey, "ttl", ttl, "attempt", attempt+1, "is_flush", isFlushLock)
			return cacheLock, nil
		}
		lastErr = err

		// For flush locks, don't retry on lock contention
		if isFlushLock && errors.Is(err, cache.ErrLockNotAcquired) {
			lma.log.Debug("Flush lock acquisition failed, not retrying",
				"key", lockKey, "error", err)
			// Return immediately for flush locks on contention
			return nil, fmt.Errorf("%w: lock already held for key %s", memcore.ErrLockAcquisitionFailed, lockKey)
		}

		if !lma.shouldRetry(ctx, attempt, maxRetries) {
			break
		}
		if retryErr := lma.waitForRetry(ctx, attempt, lockKey, maxRetries, err); retryErr != nil {
			return nil, retryErr
		}
	}
	// For flush locks that failed on first attempt, report 0 retries
	actualRetries := maxRetries
	if isFlushLock && errors.Is(lastErr, cache.ErrLockNotAcquired) {
		actualRetries = 0
	}
	return nil, lma.formatAcquisitionError(lockKey, actualRetries, lastErr)
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
	if attempt < 0 {
		attempt = 0 // Ensure non-negative
	}
	// Calculate delay with overflow protection
	// For attempts > 30, 1<<attempt would overflow, so we cap the shift operation
	shiftAmount := attempt
	if shiftAmount > 30 {
		// 2^30 * 50ms ≈ 53687 seconds ≈ 14.9 hours - more than reasonable max
		shiftAmount = 30
	}
	delay := time.Duration(1<<shiftAmount) * baseDelay
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

	// Wrap with appropriate core error type
	wrappedErr := fmt.Errorf("failed to acquire distributed lock for key %s after %d attempts: %w",
		lockKey, maxRetries+1, lastErr)

	// Check if the underlying error is ErrLockNotAcquired from cache layer
	if errors.Is(lastErr, cache.ErrLockNotAcquired) {
		// Wrap with our core lock acquisition error
		return fmt.Errorf("%w: %v", memcore.ErrLockAcquisitionFailed, wrappedErr)
	}

	return wrappedErr
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
	if resourceCfg.Model != "" {
		model = resourceCfg.Model
	}
	// Use provider configuration if available
	tokenCounter, err := mm.getOrCreateTokenCounterWithConfig(model, resourceCfg.TokenProvider)
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
) (memcore.FlushStrategy, error) {
	factory := mm.createStrategyFactory(tokenManager)
	strategyConfig := mm.getStrategyConfig(resourceCfg)

	if err := factory.ValidateStrategyConfig(strategyConfig); err != nil {
		return nil, fmt.Errorf("invalid strategy configuration for resource '%s': %w", resourceCfg.ID, err)
	}

	if strategyConfig.Type == memcore.HybridSummaryFlushing {
		return mm.createLegacyHybridStrategy(resourceCfg, tokenManager, strategyConfig)
	}

	return mm.createStrategyWithFactory(factory, resourceCfg, strategyConfig)
}

// createStrategyFactory creates and configures a strategy factory with the appropriate token counter
func (mm *Manager) createStrategyFactory(tokenManager *TokenMemoryManager) *strategies.StrategyFactory {
	var coreTokenCounter memcore.TokenCounter
	if tokenManager != nil {
		coreTokenCounter = tokenManager.GetTokenCounter()
	}
	return strategies.NewStrategyFactoryWithTokenCounter(coreTokenCounter)
}

// getStrategyConfig extracts or creates a default strategy configuration from the resource
func (mm *Manager) getStrategyConfig(resourceCfg *memcore.Resource) *memcore.FlushingStrategyConfig {
	if resourceCfg.FlushingStrategy != nil {
		return resourceCfg.FlushingStrategy
	}
	// Default to FIFO strategy
	return &memcore.FlushingStrategyConfig{
		Type:               memcore.SimpleFIFOFlushing,
		SummarizeThreshold: 0.8,
	}
}

// createStrategyWithFactory creates a strategy using the factory with appropriate options
func (mm *Manager) createStrategyWithFactory(
	factory *strategies.StrategyFactory,
	resourceCfg *memcore.Resource,
	strategyConfig *memcore.FlushingStrategyConfig,
) (memcore.FlushStrategy, error) {
	opts := mm.createStrategyOptions(resourceCfg)
	strategy, err := factory.CreateStrategy(strategyConfig, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create flushing strategy for resource '%s': %w", resourceCfg.ID, err)
	}
	return strategy, nil
}

// createLegacyHybridStrategy creates the legacy HybridFlushingStrategy for backward compatibility
func (mm *Manager) createLegacyHybridStrategy(
	resourceCfg *memcore.Resource,
	tokenManager *TokenMemoryManager,
	strategyConfig *memcore.FlushingStrategyConfig,
) (memcore.FlushStrategy, error) {
	var summarizer MessageSummarizer
	tokenCounter, err := mm.getOrCreateTokenCounter(DefaultTokenCounterModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create token counter for summarizer: %w", err)
	}
	summarizer = NewRuleBasedSummarizer(tokenCounter, 1, 1)

	flushingStrategy, err := NewHybridFlushingStrategy(strategyConfig, summarizer, tokenManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create hybrid flushing strategy for resource '%s': %w", resourceCfg.ID, err)
	}
	return flushingStrategy, nil
}

// createStrategyOptions creates strategy options based on resource configuration
func (mm *Manager) createStrategyOptions(resourceCfg *memcore.Resource) *strategies.StrategyOptions {
	opts := strategies.GetDefaultStrategyOptions()

	// Configure based on resource type and limits
	if resourceCfg.MaxTokens > 0 {
		opts.MaxTokens = resourceCfg.MaxTokens
	}

	if resourceCfg.MaxMessages > 0 {
		opts.CacheSize = resourceCfg.MaxMessages
	}

	// Set threshold from flushing strategy config if available
	if resourceCfg.FlushingStrategy != nil && resourceCfg.FlushingStrategy.SummarizeThreshold > 0 {
		opts.DefaultThreshold = resourceCfg.FlushingStrategy.SummarizeThreshold
	}

	return opts
}

// createEvictionPolicy creates an eviction policy for the given resource configuration
func (mm *Manager) createEvictionPolicy(resourceCfg *memcore.Resource) instance.EvictionPolicy {
	// Use configured eviction policy or get default
	evictionConfig := resourceCfg.GetEffectiveEvictionPolicy()
	return eviction.CreatePolicyWithConfig(evictionConfig)
}

// createMemoryInstance creates the final memory instance with all components
func (mm *Manager) createMemoryInstance(
	ctx context.Context,
	sanitizedKey, projectIDVal string,
	resourceCfg *memcore.Resource,
	components *memoryComponents,
) (memcore.Memory, error) {
	// Build token counter for this instance
	model := resourceCfg.Model
	if model == "" {
		model = DefaultTokenCounterModel
	}
	tokenCounter, err := mm.getOrCreateTokenCounterWithConfig(model, resourceCfg.TokenProvider)
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
		WithEvictionPolicy(components.evictionPolicy).
		WithTemporalClient(mm.temporalClient).
		WithTemporalTaskQueue(mm.temporalTaskQueue).
		WithPrivacyManager(mm.privacyManager).
		WithLogger(mm.log)
	if err := instanceBuilder.Validate(ctx); err != nil {
		return nil, fmt.Errorf("instance builder validation failed: %w", err)
	}
	memInstance, err := instanceBuilder.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build memory instance: %w", err)
	}
	return memInstance, nil
}
