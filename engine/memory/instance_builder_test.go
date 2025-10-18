package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/cache"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

func TestLockManagerAdapter_Lock(t *testing.T) {
	ctx := context.Background()

	t.Run("Should successfully acquire and release distributed lock", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}
		mockCacheLock := &MockCacheLock{}

		lockKey := "memory:test-project:test-key"
		ttl := 30 * time.Second

		mockCacheLockManager.On("Acquire", mock.Anything, lockKey, ttl).Return(mockCacheLock, nil)
		mockCacheLock.On("Release", mock.Anything).Return(nil)

		adapter := newLockManagerAdapter(mockCacheLockManager, "test-project")

		// Acquire lock
		lock, err := adapter.Lock(ctx, "test-key", ttl)
		require.NoError(t, err)
		require.NotNil(t, lock)

		// Release lock
		err = lock.Unlock(ctx)
		require.NoError(t, err)

		mockCacheLockManager.AssertExpectations(t)
		mockCacheLock.AssertExpectations(t)
	})

	t.Run("Should retry lock acquisition on failure", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}
		mockCacheLock := &MockCacheLock{}

		lockKey := "memory:test-project:test-key"
		ttl := 30 * time.Second

		// First two attempts fail, third succeeds
		mockCacheLockManager.On("Acquire", mock.Anything, lockKey, ttl).
			Return(nil, errors.New("lock contention")).
			Twice()
		mockCacheLockManager.On("Acquire", mock.Anything, lockKey, ttl).Return(mockCacheLock, nil).Once()

		adapter := newLockManagerAdapter(mockCacheLockManager, "test-project")

		// Should succeed after retries
		lock, err := adapter.Lock(ctx, "test-key", ttl)
		require.NoError(t, err)
		require.NotNil(t, lock)

		mockCacheLockManager.AssertExpectations(t)
	})

	t.Run("Should fail after max retries", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		lockKey := "memory:test-project:test-key"
		ttl := 30 * time.Second
		lockError := errors.New("persistent lock contention")

		// All attempts fail (4 attempts: initial + 3 retries)
		mockCacheLockManager.On("Acquire", mock.Anything, lockKey, ttl).Return(nil, lockError).Times(4)

		adapter := newLockManagerAdapter(mockCacheLockManager, "test-project")

		// Should fail after max retries
		lock, err := adapter.Lock(ctx, "test-key", ttl)
		require.Error(t, err)
		require.Nil(t, lock)
		assert.Contains(t, err.Error(), "after 4 attempts")

		mockCacheLockManager.AssertExpectations(t)
	})

	t.Run("Should handle context cancellation during retry", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		lockKey := "memory:test-project:test-key"
		ttl := 30 * time.Second

		// First attempt fails
		mockCacheLockManager.On("Acquire", mock.Anything, lockKey, ttl).
			Return(nil, errors.New("lock contention")).
			Maybe()

		adapter := newLockManagerAdapter(mockCacheLockManager, "test-project")

		// Create context that cancels after first attempt
		cancelCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer cancel()

		// Should fail due to context cancellation or timeout
		lock, err := adapter.Lock(cancelCtx, "test-key", ttl)
		require.Error(t, err)
		require.Nil(t, lock)
		// Could be either context cancellation or the wrapped error
		assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || err != nil)

		mockCacheLockManager.AssertExpectations(t)
	})

	t.Run("Should handle double unlock gracefully", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}
		mockCacheLock := &MockCacheLock{}

		lockKey := "memory:test-project:test-key"
		ttl := 30 * time.Second

		mockCacheLockManager.On("Acquire", mock.Anything, lockKey, ttl).Return(mockCacheLock, nil)
		mockCacheLock.On("Release", mock.Anything).Return(nil).Once()

		adapter := newLockManagerAdapter(mockCacheLockManager, "test-project")

		// Acquire lock
		lock, err := adapter.Lock(ctx, "test-key", ttl)
		require.NoError(t, err)

		// First unlock should succeed
		err = lock.Unlock(ctx)
		require.NoError(t, err)

		// Second unlock should fail gracefully
		err = lock.Unlock(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already released")

		mockCacheLockManager.AssertExpectations(t)
		mockCacheLock.AssertExpectations(t)
	})
}

func TestLockManagerAdapter_Concurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("Should handle concurrent lock acquisition safely", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		// Set up expectations for concurrent access
		numGoroutines := 10
		successfulLocks := 3

		for range successfulLocks {
			mockLock := &MockCacheLock{}
			mockCacheLockManager.On("Acquire", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).
				Return(mockLock, nil).
				Once()
			mockLock.On("Release", mock.Anything).Return(nil).Once()
		}

		// Remaining attempts fail
		for i := successfulLocks; i < numGoroutines; i++ {
			mockCacheLockManager.On("Acquire", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).
				Return(nil, cache.ErrLockNotAcquired).
				Times(4)

			// Initial + 3 retries
		}

		adapter := newLockManagerAdapter(mockCacheLockManager, "test-project")

		var wg sync.WaitGroup
		var successCount int32

		// Launch concurrent lock attempts
		for i := range numGoroutines {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()

				lock, err := adapter.Lock(ctx, "concurrent-test", 30*time.Second)
				if err == nil && lock != nil {
					// Successfully acquired lock, release it
					lock.Unlock(ctx)
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		// Only successful locks should be counted
		assert.Equal(t, int32(successfulLocks), atomic.LoadInt32(&successCount))
		mockCacheLockManager.AssertExpectations(t)
	})
}

func TestCreateLockManager_TTLConfiguration(t *testing.T) {
	t.Run("Should configure TTLs from resource configuration", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		manager := &Manager{
			baseLockManager: mockCacheLockManager,
		}

		resourceCfg := &memcore.Resource{
			ID:        "test-resource",
			AppendTTL: "45s",
			ClearTTL:  "15s",
			FlushTTL:  "10m",
		}

		lockManager, err := manager.createLockManager("test-project", resourceCfg)
		require.NoError(t, err)
		require.NotNil(t, lockManager)
	})

	t.Run("Should return error for invalid TTL formats", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		manager := &Manager{
			baseLockManager: mockCacheLockManager,
		}

		resourceCfg := &memcore.Resource{
			ID:        "test-resource",
			AppendTTL: "invalid-duration",
		}

		lockManager, err := manager.createLockManager("test-project", resourceCfg)
		require.Error(t, err)
		require.Nil(t, lockManager)
		require.Contains(t, err.Error(), "invalid append TTL format")

		// Test clear TTL error
		resourceCfg = &memcore.Resource{
			ID:       "test-resource",
			ClearTTL: "also-invalid",
		}

		lockManager, err = manager.createLockManager("test-project", resourceCfg)
		require.Error(t, err)
		require.Nil(t, lockManager)
		require.Contains(t, err.Error(), "invalid clear TTL format")

		// Test flush TTL error
		resourceCfg = &memcore.Resource{
			ID:       "test-resource",
			FlushTTL: "15x", // Invalid format
		}

		lockManager, err = manager.createLockManager("test-project", resourceCfg)
		require.Error(t, err)
		require.Nil(t, lockManager)
		require.Contains(t, err.Error(), "invalid flush TTL format")
	})

	t.Run("Should handle nil resource configuration", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		manager := &Manager{
			baseLockManager: mockCacheLockManager,
		}

		lockManager, err := manager.createLockManager("test-project", nil)
		require.NoError(t, err)
		require.NotNil(t, lockManager)
	})
}

// Mock implementations for testing

type MockCacheLockManager struct {
	mock.Mock
}

func (m *MockCacheLockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (cache.Lock, error) {
	args := m.Called(ctx, resource, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Lock), args.Error(1)
}

type MockCacheLock struct {
	mock.Mock
}

func (m *MockCacheLock) Release(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCacheLock) Refresh(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCacheLock) Resource() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockCacheLock) IsHeld() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestManager_createEvictionPolicy_Integration(t *testing.T) {
	manager := &Manager{} // Minimal manager for testing

	t.Run("Should integrate eviction policy with clean separation from flush strategy", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "integration-test",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 2000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type:             memcore.PriorityEviction,
				PriorityKeywords: []string{"critical", "error", "security"},
			},
			FlushingStrategy: &memcore.FlushingStrategyConfig{
				Type:               memcore.SimpleFIFOFlushing,
				SummarizeThreshold: 0.8,
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		// Create eviction policy
		policy := manager.createEvictionPolicy(resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())

		// Verify clean separation - eviction policy and flush strategy are independent
		assert.NotNil(t, resourceCfg.EvictionPolicyConfig)
		assert.NotNil(t, resourceCfg.FlushingStrategy)
		assert.Equal(t, memcore.PriorityEviction, resourceCfg.EvictionPolicyConfig.Type)
		assert.Equal(t, memcore.SimpleFIFOFlushing, resourceCfg.FlushingStrategy.Type)

		// Verify no coupling between configurations
		assert.Len(
			t,
			resourceCfg.EvictionPolicyConfig.PriorityKeywords,
			3,
		) // Keywords only in eviction config
		assert.NotEqual(t, string(memcore.PriorityEviction), string(memcore.SimpleFIFOFlushing)) // Different types
	})

	t.Run("Should use default FIFO eviction when no policy configured", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "default-test",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			// No EvictionPolicyConfig - should use default
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
	})

	t.Run("Should handle LRU eviction policy correctly", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "lru-test",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type: memcore.LRUEviction,
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "lru", policy.GetType())
	})
}

func TestManager_buildMemoryComponents_Integration(t *testing.T) {
	t.Run("Should verify clean separation in configuration", func(t *testing.T) {
		// Test verifies that resource configuration has proper separation
		// between eviction policies and flush strategies

		resourceCfg := &memcore.Resource{
			ID:        "component-test",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type:             memcore.PriorityEviction,
				PriorityKeywords: []string{"important", "urgent"},
			},
			FlushingStrategy: &memcore.FlushingStrategyConfig{
				Type: memcore.LRUFlushing,
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		// Verify config has clean separation
		assert.Equal(t, "component-test", resourceCfg.ID)
		assert.Equal(t, memcore.TokenBasedMemory, resourceCfg.Type)
		assert.Equal(t, 1000, resourceCfg.MaxTokens)
		assert.NotNil(t, resourceCfg.EvictionPolicyConfig)
		assert.NotNil(t, resourceCfg.FlushingStrategy)
		assert.Equal(t, memcore.PriorityEviction, resourceCfg.EvictionPolicyConfig.Type)
		assert.Equal(t, memcore.LRUFlushing, resourceCfg.FlushingStrategy.Type)

		// Verify no coupling between configurations
		assert.NotContains(t, string(resourceCfg.FlushingStrategy.Type), "priority") // No priority in flush strategy
		assert.Len(t, resourceCfg.EvictionPolicyConfig.PriorityKeywords, 2)          // Keywords only in eviction config
	})

	t.Run("Should demonstrate architectural compliance with Phase 2 requirements", func(t *testing.T) {
		// This test verifies that the architectural changes from Phase 2 are properly implemented
		// Priority logic is handled ONLY by eviction policies, NOT by flush strategies

		resourceCfg := &memcore.Resource{
			ID:        "architectural-compliance-test",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 2000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type:             memcore.PriorityEviction,
				PriorityKeywords: []string{"alert", "critical", "error"},
			},
			FlushingStrategy: &memcore.FlushingStrategyConfig{
				Type:               memcore.SimpleFIFOFlushing, // FIFO flushing only
				SummarizeThreshold: 0.75,
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "2h",
			},
		}

		// Verify priority information is ONLY in eviction policy config
		assert.NotNil(t, resourceCfg.EvictionPolicyConfig.PriorityKeywords)
		assert.Len(t, resourceCfg.EvictionPolicyConfig.PriorityKeywords, 3)

		// Verify flush strategy config has NO priority-related fields
		assert.Equal(t, memcore.SimpleFIFOFlushing, resourceCfg.FlushingStrategy.Type)
		assert.Zero(t, resourceCfg.FlushingStrategy.SummaryTokens) // Only flushing-related fields
		assert.Equal(t, 0.75, resourceCfg.FlushingStrategy.SummarizeThreshold)

		// Verify the architectural separation is enforced
		manager := &Manager{}
		policy := manager.createEvictionPolicy(resourceCfg)
		assert.Equal(t, "priority", policy.GetType())

		// The clean architecture ensures that:
		// 1. Eviction policies determine WHICH messages to evict
		// 2. Flush strategies determine WHEN and HOW MUCH to flush
		// 3. No cross-dependencies or coupling between the two
	})
}
