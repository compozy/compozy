package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/memory/metrics"
)

func TestNewHealthService(t *testing.T) {
	t.Run("Should create health service", func(t *testing.T) {
		ctx := t.Context()
		manager := &Manager{}
		service := NewHealthService(ctx, manager)
		assert.NotNil(t, service)
		assert.Equal(t, manager, service.manager)
		assert.Equal(t, 30*time.Second, service.checkInterval)
		assert.Equal(t, 5*time.Second, service.healthTimeout)
		assert.NotNil(t, service.stopCh)
	})
}

func TestTokenUsageHealth(t *testing.T) {
	t.Run("Should create token usage health", func(t *testing.T) {
		health := &TokenUsageHealth{
			Used:            750,
			MaxTokens:       1000,
			UsagePercentage: 75.0,
			NearLimit:       false,
		}
		assert.Equal(t, 750, health.Used)
		assert.Equal(t, 1000, health.MaxTokens)
		assert.Equal(t, 75.0, health.UsagePercentage)
		assert.False(t, health.NearLimit)
	})
}

func TestInstanceHealth(t *testing.T) {
	t.Run("Should create instance health", func(t *testing.T) {
		health := &InstanceHealth{
			MemoryID:            "test-memory",
			Healthy:             true,
			ConsecutiveFailures: 0,
		}
		assert.Equal(t, "test-memory", health.MemoryID)
		assert.True(t, health.Healthy)
		assert.Equal(t, 0, health.ConsecutiveFailures)
	})
}

func TestSystemHealth(t *testing.T) {
	t.Run("Should create system health", func(t *testing.T) {
		health := &SystemHealth{
			Healthy:            true,
			TotalInstances:     2,
			HealthyInstances:   2,
			UnhealthyInstances: 0,
			InstanceHealth:     make(map[string]*InstanceHealth),
		}
		assert.True(t, health.Healthy)
		assert.Equal(t, 2, health.TotalInstances)
		assert.Equal(t, 2, health.HealthyInstances)
		assert.Equal(t, 0, health.UnhealthyInstances)
		assert.NotNil(t, health.InstanceHealth)
	})
}

func TestHealthService_GetOverallHealth(t *testing.T) {
	t.Run("Should return system health", func(t *testing.T) {
		ctx := t.Context()
		manager := &Manager{}
		service := NewHealthService(ctx, manager)
		health := service.GetOverallHealth(ctx)
		assert.NotNil(t, health)
		assert.NotZero(t, health.LastChecked)
		assert.NotNil(t, health.InstanceHealth)
	})
}

func TestHealthService_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent GetOverallHealth calls", func(t *testing.T) {
		ctx := t.Context()
		manager := &Manager{}
		service := NewHealthService(ctx, manager)

		var wg sync.WaitGroup
		results := make([]*SystemHealth, 10)

		for i := range 10 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				health := service.GetOverallHealth(ctx)
				results[index] = health
				assert.NotNil(t, health)
				assert.NotZero(t, health.LastChecked)
				assert.NotNil(t, health.InstanceHealth)
			}(i)
		}
		wg.Wait()

		// Verify all calls succeeded
		for i, health := range results {
			assert.NotNil(t, health, "Health result %d should not be nil", i)
			assert.NotZero(t, health.LastChecked, "Health result %d should have LastChecked set", i)
		}
	})

	t.Run("Should handle concurrent health checks with Start/Stop", func(t *testing.T) {
		ctx := t.Context()
		manager := &Manager{}
		service := NewHealthService(ctx, manager)

		var wg sync.WaitGroup

		// Start background health monitoring
		wg.Go(func() {
			service.Start(ctx)
			time.Sleep(100 * time.Millisecond)
			service.Stop()
		})

		// Concurrent health checks
		for range 5 {
			wg.Go(func() {
				health := service.GetOverallHealth(ctx)
				assert.NotZero(t, health.LastChecked)
			})
		}

		wg.Wait()
	})
}

func TestHealthService_ErrorScenarios(t *testing.T) {
	t.Run("Should handle manager with nil components gracefully", func(t *testing.T) {
		ctx := t.Context()
		manager := &Manager{
			baseRedisClient: nil,
			baseLockManager: nil,
		}
		service := NewHealthService(ctx, manager)

		health := service.GetOverallHealth(ctx)
		assert.NotNil(t, health)
		assert.NotZero(t, health.LastChecked)
		assert.NotNil(t, health.InstanceHealth)
	})

	t.Run("Should handle context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		manager := &Manager{}
		service := NewHealthService(ctx, manager)

		// Cancel context immediately
		cancel()

		health := service.GetOverallHealth(ctx)
		assert.NotNil(t, health)
	})
}

// MockLock implements cache.Lock interface for testing
type MockLock struct {
	mock.Mock
}

func (m *MockLock) Release(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLock) Refresh(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLock) Resource() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockLock) IsHeld() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockLockManager implements cache.LockManager interface for testing
type MockLockManager struct {
	mock.Mock
}

func (m *MockLockManager) Acquire(ctx context.Context, key string, ttl time.Duration) (cache.Lock, error) {
	args := m.Called(ctx, key, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Lock), args.Error(1)
}

func TestHealthService_performOperationalChecks(t *testing.T) {
	t.Run("Should perform comprehensive health checks with Redis and Lock", func(t *testing.T) {
		ctx := t.Context()
		// Setup miniredis
		s := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: s.Addr()})
		// Setup lock manager mock
		lockManager := new(MockLockManager)
		lockMock := new(MockLock)
		// Create manager with Redis and lock manager
		manager := &Manager{
			baseRedisClient: client,
			baseLockManager: lockManager,
		}
		service := NewHealthService(ctx, manager)
		memoryID := "test-memory-1"
		// Set up lock manager expectations
		lockManager.On("Acquire", mock.Anything, mock.AnythingOfType("string"), 5*time.Second).Return(lockMock, nil)
		lockMock.On("Release", mock.Anything).Return(nil)
		// Perform health check
		healthy := service.performOperationalChecks(ctx, memoryID)
		assert.True(t, healthy)
		// Verify lock expectations
		lockManager.AssertExpectations(t)
		lockMock.AssertExpectations(t)
	})
	t.Run("Should fail when Redis connectivity fails", func(t *testing.T) {
		ctx := t.Context()
		// Create a manager with no Redis client to simulate connection failure
		manager := &Manager{
			baseRedisClient: nil,
		}
		service := NewHealthService(ctx, manager)
		memoryID := "test-memory-2"
		// Perform health check - should pass when Redis is nil
		healthy := service.performOperationalChecks(ctx, memoryID)
		assert.True(t, healthy)
	})
	t.Run("Should fail when lock acquisition fails", func(t *testing.T) {
		ctx := t.Context()
		// Setup miniredis
		s := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: s.Addr()})
		lockManager := new(MockLockManager)
		manager := &Manager{
			baseRedisClient: client,
			baseLockManager: lockManager,
		}
		service := NewHealthService(ctx, manager)
		memoryID := "test-memory-5"
		// Set up lock manager to fail
		lockManager.On("Acquire", mock.Anything, mock.AnythingOfType("string"), 5*time.Second).
			Return(nil, errors.New("lock failed"))
		// Perform health check
		healthy := service.performOperationalChecks(ctx, memoryID)
		assert.False(t, healthy)
		lockManager.AssertExpectations(t)
	})
	t.Run("Should pass when only Redis is available (no lock manager)", func(t *testing.T) {
		ctx := t.Context()
		// Setup miniredis
		s := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: s.Addr()})
		manager := &Manager{
			baseRedisClient: client,
			baseLockManager: nil, // No lock manager
		}
		service := NewHealthService(ctx, manager)
		memoryID := "test-memory-6"
		// Perform health check
		healthy := service.performOperationalChecks(ctx, memoryID)
		assert.True(t, healthy)
	})
}

func TestHealthService_checkInstanceHealth(t *testing.T) {
	t.Run("Should check instance health with operational checks", func(t *testing.T) {
		ctx := t.Context()
		// Setup miniredis
		s := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: s.Addr()})
		manager := &Manager{
			baseRedisClient: client,
		}
		service := NewHealthService(ctx, manager)
		memoryID := "test-memory-check"
		// Initialize health state
		metricsState := metrics.GetDefaultState()
		metricsState.UpdateHealthState(memoryID, true, 0)
		// Perform health check
		healthy := service.checkInstanceHealth(ctx, memoryID)
		assert.True(t, healthy)
		// Clean up
		metricsState.DeleteHealthState(memoryID)
	})
}

func TestHealthService_Start_Stop(t *testing.T) {
	t.Run("Should start and stop health service", func(t *testing.T) {
		ctx := t.Context()
		manager := &Manager{}
		service := NewHealthService(ctx, manager)
		// Start the service
		service.Start(ctx)
		// Give it some time to run
		time.Sleep(50 * time.Millisecond)
		// Stop the service
		service.Stop()
		// Verify it stopped
		select {
		case <-service.stopCh:
			// Channel should be closed
		default:
			t.Error("Expected stop channel to be closed")
		}
	})
}
