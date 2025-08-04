package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestInitMetrics(t *testing.T) {
	t.Run("Should initialize auth metrics successfully", func(t *testing.T) {
		// Reset metrics state
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)

		assert.NoError(t, err)
		assert.NotNil(t, authRequestsTotal)
		assert.NotNil(t, authLatency)
	})

	t.Run("Should handle nil meter gracefully", func(t *testing.T) {
		// Reset metrics state
		ResetMetricsForTesting()

		err := InitMetrics(nil)

		assert.NoError(t, err)
	})

	t.Run("Should initialize metrics only once", func(t *testing.T) {
		// Reset metrics state
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")

		// Call multiple times
		err1 := InitMetrics(meter)
		err2 := InitMetrics(meter)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, authRequestsTotal)
		assert.NotNil(t, authLatency)
	})
}

func TestRecordAuthAttempt(t *testing.T) {
	t.Run("Should handle metrics recording with initialized counters", func(t *testing.T) {
		// Reset and initialize metrics
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		assert.NoError(t, err)
		assert.NotNil(t, authRequestsTotal, "Auth requests counter should be initialized")
		assert.NotNil(t, authLatency, "Auth latency histogram should be initialized")

		ctx := context.Background()
		duration := 5 * time.Millisecond

		// Verify that metrics recording doesn't fail - business logic validation
		RecordAuthAttempt(ctx, "success", duration)
		RecordAuthAttempt(ctx, "fail", duration)
	})

	t.Run("Should handle metrics recording when counters are nil", func(t *testing.T) {
		// Ensure metrics are nil - testing resilience to uninitialized state
		ResetMetricsForTesting()

		ctx := context.Background()
		duration := 1 * time.Millisecond

		// Business logic: system should be resilient to uninitialized metrics
		// This validates graceful degradation behavior
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("RecordAuthAttempt should not panic when metrics are nil, got panic: %v", r)
			}
		}()
		RecordAuthAttempt(ctx, "success", duration)
	})

	t.Run("Should process various authentication status types", func(t *testing.T) {
		// Reset and initialize metrics
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		assert.NoError(t, err)

		ctx := context.Background()
		duration := 2 * time.Millisecond

		// Business logic: system should handle all authentication outcome types
		statuses := []string{"success", "fail", "timeout", "error", "invalid_token", "expired"}
		for _, status := range statuses {
			RecordAuthAttempt(ctx, status, duration)
		}
		// Validates that metrics system can handle diverse auth scenarios
	})
}
