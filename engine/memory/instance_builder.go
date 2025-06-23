package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance"
	"github.com/compozy/compozy/engine/memory/store"
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
	lockManager, err := mm.createLockManager(projectIDVal)
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

// createLockManager creates a memory lock manager with project namespacing
//
//nolint:unparam // error return is intentional for future extensibility
func (mm *Manager) createLockManager(_ string) (*instance.LockManagerImpl, error) {
	// Convert LockManager to Locker interface
	// TODO: For now, use a simple wrapper until proper interface is established
	locker := &lockManagerAdapter{manager: mm.baseLockManager}
	lockManager := instance.NewLockManager(locker)
	return lockManager, nil
}

// lockManagerAdapter adapts cache.LockManager to instance.Locker
type lockManagerAdapter struct {
	manager cache.LockManager
}

// Lock implements instance.Locker
func (lma *lockManagerAdapter) Lock(_ context.Context, key string, _ time.Duration) (instance.Lock, error) {
	// For now, return a simple implementation
	// TODO: This will need proper implementation when cache interfaces are stabilized
	return &simpleLock{key: key}, nil
}

// simpleLock is a simple lock implementation for now
type simpleLock struct {
	key string
}

// Unlock implements instance.Lock
func (sl *simpleLock) Unlock(_ context.Context) error {
	// TODO: For now, just return nil - proper implementation needed later
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
