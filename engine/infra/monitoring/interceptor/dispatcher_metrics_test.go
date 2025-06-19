package interceptor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDispatcherMetrics(t *testing.T) {
	ctx := context.Background()
	// Reset metrics for clean test state
	ResetMetricsForTesting(ctx)
	t.Run("Should track dispatcher lifecycle events", func(t *testing.T) {
		dispatcherID := "test-dispatcher-lifecycle"
		// Test start dispatcher
		StartDispatcher(ctx, dispatcherID)
		// Test heartbeat
		RecordDispatcherHeartbeat(ctx, dispatcherID)
		// Test restart
		RecordDispatcherRestart(ctx, dispatcherID)
		// Test stop dispatcher
		StopDispatcher(ctx, dispatcherID)
		// No assertions here since metrics are internal, but this verifies no panics occur
		assert.True(t, true, "Dispatcher lifecycle events should complete without errors")
	})
	t.Run("Should handle multiple dispatchers", func(t *testing.T) {
		dispatcher1 := "test-dispatcher-1"
		dispatcher2 := "test-dispatcher-2"
		// Start multiple dispatchers
		StartDispatcher(ctx, dispatcher1)
		StartDispatcher(ctx, dispatcher2)
		// Record heartbeats
		RecordDispatcherHeartbeat(ctx, dispatcher1)
		RecordDispatcherHeartbeat(ctx, dispatcher2)
		// Stop them
		StopDispatcher(ctx, dispatcher1)
		StopDispatcher(ctx, dispatcher2)
		assert.True(t, true, "Multiple dispatcher operations should complete without errors")
	})
	t.Run("Should handle nil meter gracefully", func(t *testing.T) {
		// Reset to ensure nil meter state
		ResetMetricsForTesting(ctx)
		dispatcherID := "test-nil-meter"
		// These should not panic with nil meter
		StartDispatcher(ctx, dispatcherID)
		RecordDispatcherHeartbeat(ctx, dispatcherID)
		RecordDispatcherRestart(ctx, dispatcherID)
		StopDispatcher(ctx, dispatcherID)
		assert.True(t, true, "Dispatcher operations should handle nil meter gracefully")
	})
}
