package monitoring_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	monitoringinterceptor "github.com/compozy/compozy/engine/infra/monitoring/interceptor"
)

func TestTemporalMetricsIntegration(t *testing.T) {
	t.Run("Should track worker metrics", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Initialize temporal interceptor to register metrics
		_ = env.monitoring.TemporalInterceptor(t.Context())
		// Set configured workers count
		monitoringinterceptor.SetConfiguredWorkerCount(5)
		defer monitoringinterceptor.SetConfiguredWorkerCount(0) // Reset to default
		// Simulate workers starting and stopping
		ctx := context.Background()
		monitoringinterceptor.IncrementRunningWorkers(ctx)
		monitoringinterceptor.IncrementRunningWorkers(ctx)
		monitoringinterceptor.IncrementRunningWorkers(ctx)
		// Wait for metrics to be recorded
		assert.Eventually(t, func() bool {
			metrics, err := env.GetMetrics()
			if err != nil {
				return false
			}
			return strings.Contains(metrics, "compozy_temporal_workers_configured_total{") &&
				strings.Contains(metrics, "} 5") &&
				strings.Contains(metrics, "compozy_temporal_workers_running_total{") &&
				strings.Contains(metrics, "} 3")
		}, 2*time.Second, 25*time.Millisecond)
		// Simulate workers stopping
		monitoringinterceptor.DecrementRunningWorkers(ctx)
		monitoringinterceptor.DecrementRunningWorkers(ctx)
		// Wait for metrics to be updated
		assert.Eventually(t, func() bool {
			metrics, err := env.GetMetrics()
			if err != nil {
				return false
			}
			return strings.Contains(metrics, "compozy_temporal_workers_running_total{") &&
				strings.Contains(metrics, "} 1")
		}, 2*time.Second, 25*time.Millisecond)
	})
	t.Run("Should verify temporal interceptor exists", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Get workflow interceptor from monitoring service
		workflowInterceptor := env.monitoring.TemporalInterceptor(t.Context())
		require.NotNil(t, workflowInterceptor)
		// Verify it's not nil
		assert.NotNil(t, workflowInterceptor, "Should have a valid interceptor")
	})
	t.Run("Should have temporal metrics registered", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Initialize temporal interceptor to register metrics
		interceptor := env.monitoring.TemporalInterceptor(t.Context())
		require.NotNil(t, interceptor)
		// Get metrics and verify observable gauges are registered (these always appear)
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Observable gauges should always be exported even without data
		assert.Contains(t, metrics, "# TYPE compozy_temporal_workers_configured_total gauge")
		// Note: Counters and histograms only appear after being used, which is correct OpenTelemetry behavior
		// For this test, we just verify the interceptor was created successfully
		assert.NotNil(t, interceptor, "Temporal interceptor should be created")
	})
}
