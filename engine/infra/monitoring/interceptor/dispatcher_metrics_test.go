package interceptor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDispatcherMetrics(t *testing.T) {
	ctx := context.Background()
	// Reset metrics for clean test state
	ResetMetricsForTesting(ctx)
	t.Run("Should track dispatcher lifecycle without panics", func(t *testing.T) {
		dispatcherID := "test-dispatcher-lifecycle"
		// Verify that lifecycle operations complete successfully
		assert.NotPanics(t, func() {
			StartDispatcher(ctx, dispatcherID)
			RecordDispatcherHeartbeat(ctx, dispatcherID)
			RecordDispatcherRestart(ctx, dispatcherID)
			StopDispatcher(ctx, dispatcherID)
		})
	})
	t.Run("Should handle multiple dispatchers without panics", func(t *testing.T) {
		dispatcher1 := "test-dispatcher-1"
		dispatcher2 := "test-dispatcher-2"
		// Verify concurrent dispatcher operations don't panic
		assert.NotPanics(t, func() {
			StartDispatcher(ctx, dispatcher1)
			StartDispatcher(ctx, dispatcher2)
			RecordDispatcherHeartbeat(ctx, dispatcher1)
			RecordDispatcherHeartbeat(ctx, dispatcher2)
			StopDispatcher(ctx, dispatcher1)
			StopDispatcher(ctx, dispatcher2)
		})
	})
	t.Run("Should handle nil meter gracefully without panics", func(t *testing.T) {
		// Reset to ensure nil meter state
		ResetMetricsForTesting(ctx)
		dispatcherID := "test-nil-meter"
		// These operations should not panic even with nil meter
		assert.NotPanics(t, func() {
			StartDispatcher(ctx, dispatcherID)
			RecordDispatcherHeartbeat(ctx, dispatcherID)
			RecordDispatcherRestart(ctx, dispatcherID)
			StopDispatcher(ctx, dispatcherID)
		})
	})
	t.Run("Should record takeover metrics without panics", func(t *testing.T) {
		dispatcherID := "test-dispatcher-takeover"
		assert.NotPanics(t, func() {
			RecordDispatcherTakeover(ctx, dispatcherID, 25*time.Millisecond, "started")
		})
	})
}
