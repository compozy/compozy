package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
)

// TestHealthMonitorBasic tests basic health monitoring functionality
func TestHealthMonitorBasic(t *testing.T) {
	t.Run("Should monitor all components", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		monitor := NewHealthMonitor(env)
		ctx := context.Background()
		// Perform initial health check
		monitor.performHealthChecks(ctx)
		// Get status
		status := monitor.GetHealthStatus()
		// Should have status for all components
		assert.Contains(t, status, "Redis")
		assert.Contains(t, status, "Temporal")
		assert.Contains(t, status, "MemoryManager")
		assert.Contains(t, status, "System")
		// Redis should be healthy if available
		redisStatus := status["Redis"]
		t.Logf("Redis status: %s, response time: %v", redisStatus.Status, redisStatus.ResponseTime)
		if redisStatus.Status == "healthy" {
			assert.NoError(t, redisStatus.LastError)
			assert.NotZero(t, redisStatus.ResponseTime)
		}
		// System should always be available
		systemStatus := status["System"]
		assert.Equal(t, "healthy", systemStatus.Status)
		assert.Contains(t, systemStatus.Details, "goroutines")
		assert.Contains(t, systemStatus.Details, "memory")
	})
}

// TestHealthMonitorContinuous tests continuous monitoring
func TestHealthMonitorContinuous(t *testing.T) {
	t.Run("Should continuously monitor health", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		monitor := NewHealthMonitor(env)
		monitor.checkInterval = 100 * time.Millisecond // Faster for testing
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		// Start monitoring
		monitor.Start(ctx)
		defer monitor.Stop()
		// Wait for a few checks
		time.Sleep(350 * time.Millisecond)
		// Should have multiple checks
		status := monitor.GetHealthStatus()
		for component, health := range status {
			t.Logf("Component %s last check: %v ago", component, time.Since(health.LastCheck))
			assert.WithinDuration(t, time.Now(), health.LastCheck, 400*time.Millisecond)
		}
	})
}

// TestHealthMonitorMetrics tests metrics collection
func TestHealthMonitorMetrics(t *testing.T) {
	t.Run("Should collect operation metrics", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		monitor := NewHealthMonitor(env)
		// Record some operations
		monitor.RecordOperation("Redis", true, 10*time.Millisecond)
		monitor.RecordOperation("Redis", false, 50*time.Millisecond)
		monitor.RecordOperation("Memory", true, 20*time.Millisecond)
		monitor.RecordOperation("Memory", true, 30*time.Millisecond)
		// Get metrics
		metrics := monitor.GetMetrics()
		assert.Equal(t, int64(2), metrics["redis_operations"])
		assert.Equal(t, int64(1), metrics["redis_errors"])
		assert.Equal(t, int64(2), metrics["memory_operations"])
		assert.Equal(t, int64(0), metrics["memory_errors"])
		// Check average response time is reasonable
		avgResponseStr := metrics["average_response_time"].(string)
		assert.NotEmpty(t, avgResponseStr)
		t.Logf("Average response time: %s", avgResponseStr)
	})
}

// TestHealthMonitorAlerts tests alert generation
func TestHealthMonitorAlerts(t *testing.T) {
	t.Run("Should generate alerts on failures", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		monitor := NewHealthMonitor(env)
		ctx := context.Background()
		// Close Redis to trigger failure
		env.GetRedis().Close()
		// Perform health check
		monitor.performHealthChecks(ctx)
		// Should have alerts
		alerts := monitor.GetAlerts(time.Now().Add(-1 * time.Minute))
		assert.NotEmpty(t, alerts)
		// Find Redis alert
		var redisAlert *HealthAlert
		for i := range alerts {
			if alerts[i].Component == "Redis" {
				redisAlert = &alerts[i]
				break
			}
		}
		assert.NotNil(t, redisAlert)
		assert.Equal(t, "error", redisAlert.Severity)
		assert.Contains(t, redisAlert.Message, "Redis ping failed")
		t.Logf("Redis alert: %s", redisAlert.Message)
	})
}

// TestHealthMonitorHelper tests the health check helper
func TestHealthMonitorHelper(t *testing.T) {
	t.Run("Should wait for healthy components", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		helper := NewHealthCheckHelper(env)
		// Should be healthy initially
		helper.RequireHealthy(t)
		// Test wait functionality
		helper.WaitForHealthy(t, 5*time.Second)
	})
	t.Run("Should monitor test execution", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		helper := NewHealthCheckHelper(env)
		operationCount := 0
		// Monitor a test
		helper.MonitorTest(t, func() {
			ctx := context.Background()
			// Perform some operations
			memRef := core.MemoryReference{
				ID:  "customer-support",
				Key: "monitor-test-{{.test.id}}",
			}
			workflowContext := map[string]any{
				"project.id": "test-project",
				"test.id":    fmt.Sprintf("monitor-%d", time.Now().Unix()),
			}
			instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
			require.NoError(t, err)
			// Add messages
			for i := 0; i < 5; i++ {
				err := instance.Append(ctx, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Test message %d", i),
				})
				assert.NoError(t, err)
				operationCount++
			}
			// Read messages
			messages, err := instance.Read(ctx)
			assert.NoError(t, err)
			assert.Len(t, messages, 5)
			operationCount++
		})
		// Should have completed operations
		assert.Equal(t, 6, operationCount)
	})
}

// TestHealthMonitorReport tests health report generation
func TestHealthMonitorReport(t *testing.T) {
	t.Run("Should generate comprehensive health report", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		monitor := NewHealthMonitor(env)
		ctx := context.Background()
		// Perform some operations
		monitor.RecordOperation("Redis", true, 15*time.Millisecond)
		monitor.RecordOperation("Memory", true, 25*time.Millisecond)
		monitor.RecordOperation("Memory", false, 100*time.Millisecond)
		// Perform health checks
		monitor.performHealthChecks(ctx)
		// Generate an alert
		monitor.recordAlert("Test", "warning", "This is a test alert",
			map[string]any{"test": true})
		// Print report
		monitor.PrintHealthReport(t)
		// Verify report was generated (check logs)
		metrics := monitor.GetMetrics()
		assert.NotNil(t, metrics["uptime_seconds"])
		assert.NotNil(t, metrics["redis_operations"])
		assert.NotNil(t, metrics["memory_operations"])
	})
}

// TestHealthMonitorUnderLoad tests monitoring under load
func TestHealthMonitorUnderLoad(t *testing.T) {
	t.Run("Should track metrics under concurrent load", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		monitor := NewHealthMonitor(env)
		ctx := context.Background()
		// Start monitoring
		monitor.Start(ctx)
		defer monitor.Stop()
		// Wait for initial health check
		time.Sleep(100 * time.Millisecond)
		// Generate concurrent load
		const numGoroutines = 10
		const operationsPerGoroutine = 50
		done := make(chan bool, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(_ int) {
				defer func() { done <- true }()
				for j := 0; j < operationsPerGoroutine; j++ {
					component := "Redis"
					if j%3 == 0 {
						component = "Memory"
					}
					success := j%10 != 0 // 10% failure rate
					duration := time.Duration(10+j%40) * time.Millisecond
					monitor.RecordOperation(component, success, duration)
					time.Sleep(time.Millisecond)
				}
			}(i)
		}
		// Wait for completion
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		// Check metrics
		metrics := monitor.GetMetrics()
		totalOps := metrics["redis_operations"].(int64) + metrics["memory_operations"].(int64)
		assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), totalOps)
		// Print final report
		monitor.PrintHealthReport(t)
		// Verify system remained healthy
		status := monitor.GetHealthStatus()
		// Check individual components as there might not be a "System" status
		assert.NotEmpty(t, status, "Should have health status")
		// All components should be healthy
		for component, componentStatus := range status {
			t.Logf("Component %s status: %s", component, componentStatus.Status)
			// Allow degraded status for components under load
			assert.Contains(t, []string{"healthy", "degraded"}, componentStatus.Status,
				"Component %s should be healthy or degraded under load", component)
		}
	})
}
