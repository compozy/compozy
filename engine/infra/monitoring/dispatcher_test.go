package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatcherHealthMonitoring(t *testing.T) {
	ctx := t.Context()
	// Reset metrics for clean test state
	ResetDispatcherHealthMetricsForTesting(ctx)
	t.Run("Should register and track dispatcher health", func(t *testing.T) {
		dispatcherID := "test-dispatcher-1"
		staleThreshold := 100 * time.Millisecond
		// Register dispatcher
		RegisterDispatcher(ctx, dispatcherID, staleThreshold)
		// Check initial health
		health, exists := GetDispatcherHealth(dispatcherID)
		require.True(t, exists, "Dispatcher should exist after registration")
		assert.True(t, health.IsHealthy, "Dispatcher should be healthy initially")
		assert.Equal(t, dispatcherID, health.DispatcherID)
		assert.Equal(t, staleThreshold, health.StaleThreshold)
		// Update heartbeat
		UpdateDispatcherHeartbeat(ctx, dispatcherID)
		// Wait for stale threshold
		time.Sleep(staleThreshold + 10*time.Millisecond)
		// Check stale status
		health, exists = GetDispatcherHealth(dispatcherID)
		require.True(t, exists, "Dispatcher should still exist")
		assert.True(t, health.IsStale(), "Dispatcher should be stale after threshold")
		assert.False(t, health.IsHealthy, "Dispatcher should be unhealthy when stale")
		// Unregister dispatcher
		UnregisterDispatcher(ctx, dispatcherID)
		_, exists = GetDispatcherHealth(dispatcherID)
		assert.False(t, exists, "Dispatcher should not exist after unregistration")
	})
	t.Run("Should count healthy and stale dispatchers", func(t *testing.T) {
		// Register multiple dispatchers
		RegisterDispatcher(ctx, "healthy-1", time.Minute)
		RegisterDispatcher(ctx, "healthy-2", time.Minute)
		RegisterDispatcher(ctx, "stale-1", 10*time.Millisecond)
		// Wait for one to become stale
		time.Sleep(20 * time.Millisecond)
		// Check counts
		healthyCount := GetHealthyDispatcherCount()
		staleCount := GetStaleDispatcherCount()
		assert.Equal(t, 2, healthyCount, "Should have 2 healthy dispatchers")
		assert.Equal(t, 1, staleCount, "Should have 1 stale dispatcher")
		// Get all dispatcher health
		allHealth := GetAllDispatcherHealth()
		assert.Len(t, allHealth, 3, "Should have 3 total dispatchers")
		// Clean up
		UnregisterDispatcher(ctx, "healthy-1")
		UnregisterDispatcher(ctx, "healthy-2")
		UnregisterDispatcher(ctx, "stale-1")
	})
	t.Run("Should handle heartbeat updates correctly", func(t *testing.T) {
		dispatcherID := "heartbeat-test"
		staleThreshold := 100 * time.Millisecond
		// Register dispatcher
		RegisterDispatcher(ctx, dispatcherID, staleThreshold)
		// Wait for it to become stale
		time.Sleep(staleThreshold + 10*time.Millisecond)
		health, _ := GetDispatcherHealth(dispatcherID)
		assert.True(t, health.IsStale(), "Should be stale before heartbeat")
		// Send heartbeat to refresh
		UpdateDispatcherHeartbeat(ctx, dispatcherID)
		health, _ = GetDispatcherHealth(dispatcherID)
		assert.False(t, health.IsStale(), "Should not be stale after heartbeat")
		assert.True(t, health.IsHealthy, "Should be healthy after heartbeat")
		// Clean up
		UnregisterDispatcher(ctx, dispatcherID)
	})
}

func TestDispatcherHealthUpdateHealth(t *testing.T) {
	t.Run("Should update health status correctly", func(t *testing.T) {
		health := &DispatcherHealth{
			DispatcherID:        "test",
			LastHeartbeat:       time.Now().Add(-5 * time.Minute),
			IsHealthy:           true,
			StaleThreshold:      2 * time.Minute,
			LastHealthCheck:     time.Now().Add(-1 * time.Minute),
			ConsecutiveFailures: 0,
		}
		// Should become unhealthy
		now := time.Now()
		health.UpdateHealthAt(now)
		assert.False(t, health.IsHealthy, "Should be unhealthy when stale")
		assert.Equal(t, 1, health.ConsecutiveFailures, "Should increment failure count")
		// Update again - failure count should increment
		health.UpdateHealthAt(now)
		assert.False(t, health.IsHealthy, "Should still be unhealthy")
		assert.Equal(t, 2, health.ConsecutiveFailures, "Should increment failure count again")
		// Make it healthy again
		health.LastHeartbeat = time.Now()
		health.UpdateHealthAt(time.Now())
		assert.True(t, health.IsHealthy, "Should be healthy after recent heartbeat")
		assert.Equal(t, 0, health.ConsecutiveFailures, "Should reset failure count when healthy")
	})
	t.Run("Should correctly identify stale dispatchers", func(t *testing.T) {
		health := &DispatcherHealth{
			DispatcherID:   "test",
			LastHeartbeat:  time.Now().Add(-30 * time.Second),
			StaleThreshold: 10 * time.Second,
		}
		assert.True(t, health.IsStale(), "Should be stale when heartbeat is old")
		// Make heartbeat recent
		health.LastHeartbeat = time.Now().Add(-5 * time.Second)
		assert.False(t, health.IsStale(), "Should not be stale when heartbeat is recent")
	})
}
