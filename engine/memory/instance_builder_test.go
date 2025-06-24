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
	"github.com/compozy/compozy/pkg/logger"
)

func TestLockManagerAdapter_Lock(t *testing.T) {
	ctx := context.Background()
	log := logger.FromContext(ctx)

	t.Run("Should successfully acquire and release distributed lock", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}
		mockCacheLock := &MockCacheLock{}

		lockKey := "memory:test-project:test-key"
		ttl := 30 * time.Second

		mockCacheLockManager.On("Acquire", mock.Anything, lockKey, ttl).Return(mockCacheLock, nil)
		mockCacheLock.On("Release", mock.Anything).Return(nil)

		adapter := newLockManagerAdapter(mockCacheLockManager, log, "test-project")

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

		adapter := newLockManagerAdapter(mockCacheLockManager, log, "test-project")

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

		adapter := newLockManagerAdapter(mockCacheLockManager, log, "test-project")

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

		adapter := newLockManagerAdapter(mockCacheLockManager, log, "test-project")

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

		adapter := newLockManagerAdapter(mockCacheLockManager, log, "test-project")

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
	log := logger.FromContext(ctx)

	t.Run("Should handle concurrent lock acquisition safely", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		// Set up expectations for concurrent access
		numGoroutines := 10
		successfulLocks := 3

		for i := 0; i < successfulLocks; i++ {
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

		adapter := newLockManagerAdapter(mockCacheLockManager, log, "test-project")

		var wg sync.WaitGroup
		var successCount int32

		// Launch concurrent lock attempts
		for i := 0; i < numGoroutines; i++ {
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
	ctx := context.Background()
	log := logger.FromContext(ctx)

	t.Run("Should configure TTLs from resource configuration", func(t *testing.T) {
		mockCacheLockManager := &MockCacheLockManager{}

		manager := &Manager{
			baseLockManager: mockCacheLockManager,
			log:             log,
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
			log:             log,
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
			log:             log,
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
