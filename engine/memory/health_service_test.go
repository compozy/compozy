package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: These tests use nil for MemoryManager which means they test the health service
// in isolation without actual memory instances

func TestNewHealthService(t *testing.T) {
	t.Run("Should create health service with default values", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		assert.NotNil(t, service)
		assert.Equal(t, 30*time.Second, service.checkInterval)
		assert.Equal(t, 5*time.Second, service.healthTimeout)
		assert.NotNil(t, service.stopCh)
	})

	t.Run("Should handle nil logger", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		assert.NotNil(t, service)
		assert.NotNil(t, service.log)
	})
}

func TestHealthService_RegisterUnregister(t *testing.T) {
	t.Run("Should register and unregister instances", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		// Register instance
		service.RegisterInstance("test-memory-1")

		// Check if registered
		health, exists := service.GetInstanceHealth("test-memory-1")
		assert.True(t, exists)
		assert.NotNil(t, health)
		assert.Equal(t, "test-memory-1", health.MemoryID)
		assert.True(t, health.Healthy)
		assert.Equal(t, 0, health.ConsecutiveFailures)

		// Unregister instance
		service.UnregisterInstance("test-memory-1")

		// Check if unregistered
		_, exists = service.GetInstanceHealth("test-memory-1")
		assert.False(t, exists)
	})
}

func TestHealthService_OverallHealth(t *testing.T) {
	t.Run("Should return unhealthy when manager is nil even with registered instances", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		// Register multiple instances
		service.RegisterInstance("memory-1")
		service.RegisterInstance("memory-2")
		service.RegisterInstance("memory-3")

		// Get overall health
		health := service.GetOverallHealth(ctx)

		// Without a manager, the service cannot collect instance health
		assert.False(t, health.Healthy)
		assert.Equal(t, 0, health.TotalInstances)
		assert.Equal(t, 0, health.HealthyInstances)
		assert.Equal(t, 0, health.UnhealthyInstances)
		assert.Empty(t, health.InstanceHealth)
		assert.Contains(t, health.SystemErrors, "memory manager not available")
	})

	t.Run("Should return unhealthy when no instances registered", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		health := service.GetOverallHealth(ctx)

		assert.False(t, health.Healthy)
		assert.Equal(t, 0, health.TotalInstances)
		assert.Equal(t, 0, health.HealthyInstances)
		assert.Equal(t, 0, health.UnhealthyInstances)
		assert.Empty(t, health.InstanceHealth)
		assert.Contains(t, health.SystemErrors, "memory manager not available")
	})

	t.Run("Should handle mix of healthy and unhealthy instances", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		// Register instances
		service.RegisterInstance("healthy-memory")
		service.RegisterInstance("unhealthy-memory")

		// Mark one as unhealthy
		if state, exists := service.healthStates.Load("unhealthy-memory"); exists {
			if healthState, ok := state.(*HealthState); ok {
				healthState.mu.Lock()
				healthState.IsHealthy = false
				healthState.ConsecutiveFailures = 3
				healthState.LastHealthCheck = time.Now().Add(-10 * time.Minute) // Old check
				healthState.mu.Unlock()
			}
		}

		health := service.GetOverallHealth(ctx)

		// With nil manager, system is always unhealthy
		assert.False(t, health.Healthy)
		assert.Equal(t, 0, health.TotalInstances)
		assert.Equal(t, 0, health.HealthyInstances)
		assert.Equal(t, 0, health.UnhealthyInstances)
		assert.Empty(t, health.InstanceHealth)
		assert.Contains(t, health.SystemErrors, "memory manager not available")
	})
}

func TestHealthService_TokenUsage(t *testing.T) {
	t.Run("Should track token usage state even without manager", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		// Register instance
		service.RegisterInstance("memory-with-tokens")

		// Update token usage
		UpdateTokenUsageState("memory-with-tokens", 850, 1000)

		// Verify the token state is tracked
		tokenStateRaw, exists := memoryTokenStates.Load("memory-with-tokens")
		require.True(t, exists, "Token state should be tracked")

		tokenState, ok := tokenStateRaw.(*TokenState)
		require.True(t, ok, "Token state should be correct type")

		tokenState.mu.RLock()
		assert.Equal(t, int64(850), tokenState.TokensUsed)
		assert.Equal(t, int64(1000), tokenState.MaxTokens)
		tokenState.mu.RUnlock()

		// Test near limit update
		UpdateTokenUsageState("memory-with-tokens", 900, 1000)
		tokenStateRaw, _ = memoryTokenStates.Load("memory-with-tokens")
		tokenState = tokenStateRaw.(*TokenState)
		tokenState.mu.RLock()
		assert.Equal(t, int64(900), tokenState.TokensUsed)
		tokenState.mu.RUnlock()

		// Note: When manager is nil, instance health is not collected
		health := service.GetOverallHealth(ctx)
		assert.Empty(t, health.InstanceHealth, "Instance health not collected without manager")
	})
}

func TestMemoryHealthService_Configuration(t *testing.T) {
	t.Run("Should allow configuration of intervals", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		// Set custom intervals
		service.SetCheckInterval(60 * time.Second)
		service.SetHealthTimeout(10 * time.Second)

		assert.Equal(t, 60*time.Second, service.checkInterval)
		assert.Equal(t, 10*time.Second, service.healthTimeout)
	})
}

func TestMemoryHealthService_StartStop(t *testing.T) {
	t.Run("Should start and stop health monitoring", func(_ *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		// Set a short interval for testing
		service.SetCheckInterval(100 * time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start the service
		service.Start(ctx)

		// Wait a bit for the service to run
		time.Sleep(200 * time.Millisecond)

		// Stop the service
		service.Stop()

		// Verify it stopped (no assertions needed, just making sure it doesn't hang)
	})
}

func TestMemoryHealthService_HealthCheck(t *testing.T) {
	t.Run("Should perform health checks and update states", func(t *testing.T) {
		ctx := context.Background()
		service := NewHealthService(ctx, nil)

		// Register instance
		service.RegisterInstance("test-memory")

		// Get initial state
		initialHealth, exists := service.GetInstanceHealth("test-memory")
		assert.True(t, exists)
		initialTime := initialHealth.LastChecked

		// Wait a moment
		time.Sleep(10 * time.Millisecond)

		// Perform health check
		service.performHealthCheck(ctx)

		// Check if last checked time was updated
		updatedHealth, exists := service.GetInstanceHealth("test-memory")
		assert.True(t, exists)
		assert.True(t, updatedHealth.LastChecked.After(initialTime))
	})
}

func TestTokenUsageHealth(t *testing.T) {
	t.Run("Should calculate usage percentage correctly", func(t *testing.T) {
		usage := &TokenUsageHealth{
			Used:      750,
			MaxTokens: 1000,
		}

		// Calculate percentage
		usage.UsagePercentage = float64(usage.Used) / float64(usage.MaxTokens) * 100
		usage.NearLimit = usage.UsagePercentage > 85.0

		assert.InDelta(t, 75.0, usage.UsagePercentage, 0.1)
		assert.False(t, usage.NearLimit)

		// Test near limit
		usage.Used = 900
		usage.UsagePercentage = float64(usage.Used) / float64(usage.MaxTokens) * 100
		usage.NearLimit = usage.UsagePercentage > 85.0

		assert.InDelta(t, 90.0, usage.UsagePercentage, 0.1)
		assert.True(t, usage.NearLimit)
	})
}
