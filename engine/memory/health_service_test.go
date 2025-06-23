package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHealthService(t *testing.T) {
	t.Run("Should create health service", func(t *testing.T) {
		ctx := context.Background()
		manager := &Manager{}
		service := NewHealthService(ctx, manager)
		assert.NotNil(t, service)
		assert.Equal(t, manager, service.manager)
		assert.Equal(t, 30*time.Second, service.checkInterval)
		assert.Equal(t, 5*time.Second, service.healthTimeout)
		assert.NotNil(t, service.log)
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
		ctx := context.Background()
		manager := &Manager{}
		service := NewHealthService(ctx, manager)
		health := service.GetOverallHealth(ctx)
		assert.NotNil(t, health)
		assert.NotZero(t, health.LastChecked)
		assert.NotNil(t, health.InstanceHealth)
	})
}

func TestHealthService_Start_Stop(t *testing.T) {
	t.Run("Should start and stop health service", func(t *testing.T) {
		ctx := context.Background()
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
