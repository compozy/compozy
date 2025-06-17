package monitoring_test

import (
	"context"
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
		// Set configured workers count
		monitoringinterceptor.SetConfiguredWorkerCount(5)
		defer monitoringinterceptor.SetConfiguredWorkerCount(0) // Reset to default
		// Simulate workers starting and stopping
		ctx := context.Background()
		monitoringinterceptor.IncrementRunningWorkers(ctx)
		monitoringinterceptor.IncrementRunningWorkers(ctx)
		monitoringinterceptor.IncrementRunningWorkers(ctx)
		// Give metrics time to be recorded
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Check configured workers gauge
		assert.Contains(t, metrics, "compozy_temporal_workers_configured_total 5")
		// Check running workers gauge
		assert.Contains(t, metrics, "compozy_temporal_workers_running_total 3")
		// Simulate workers stopping
		monitoringinterceptor.DecrementRunningWorkers(ctx)
		monitoringinterceptor.DecrementRunningWorkers(ctx)
		// Give metrics time to be recorded
		time.Sleep(100 * time.Millisecond)
		// Get updated metrics
		metrics, err = env.GetMetrics()
		require.NoError(t, err)
		// Check running workers gauge decreased
		assert.Contains(t, metrics, "compozy_temporal_workers_running_total 1")
	})
	t.Run("Should verify temporal interceptor exists", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Get workflow interceptor from monitoring service
		workflowInterceptor := env.monitoring.TemporalInterceptor()
		require.NotNil(t, workflowInterceptor)
		// Verify it's not nil
		assert.NotNil(t, workflowInterceptor, "Should have a valid interceptor")
	})
	t.Run("Should have temporal metrics registered", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Get metrics and verify Temporal metrics are registered
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Check for metric type declarations (these appear even without data)
		assert.Contains(t, metrics, "# TYPE compozy_temporal_workflow_started_total counter")
		assert.Contains(t, metrics, "# TYPE compozy_temporal_workflow_completed_total counter")
		assert.Contains(t, metrics, "# TYPE compozy_temporal_workflow_failed_total counter")
		assert.Contains(t, metrics, "# TYPE compozy_temporal_workflow_task_duration_seconds histogram")
		assert.Contains(t, metrics, "# TYPE compozy_temporal_workers_configured_total gauge")
		assert.Contains(t, metrics, "# TYPE compozy_temporal_workers_running_total gauge")
	})
}
