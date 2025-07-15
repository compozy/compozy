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
	t.Run("Should record auth success metrics", func(t *testing.T) {
		// Reset and initialize metrics
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		assert.NoError(t, err)

		ctx := context.Background()
		duration := 5 * time.Millisecond

		// Should not panic - noop implementation doesn't record but validates the call
		assert.NotPanics(t, func() {
			RecordAuthAttempt(ctx, "success", duration)
		})
	})

	t.Run("Should record auth failure metrics", func(t *testing.T) {
		// Reset and initialize metrics
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		assert.NoError(t, err)

		ctx := context.Background()
		duration := 3 * time.Millisecond

		// Should not panic - noop implementation doesn't record but validates the call
		assert.NotPanics(t, func() {
			RecordAuthAttempt(ctx, "fail", duration)
		})
	})

	t.Run("Should handle nil metrics gracefully", func(t *testing.T) {
		// Ensure metrics are nil
		ResetMetricsForTesting()

		ctx := context.Background()
		duration := 1 * time.Millisecond

		// Should not panic when metrics are nil
		assert.NotPanics(t, func() {
			RecordAuthAttempt(ctx, "success", duration)
		})
	})

	t.Run("Should handle different status values", func(t *testing.T) {
		// Reset and initialize metrics
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		assert.NoError(t, err)

		ctx := context.Background()
		duration := 2 * time.Millisecond

		// Test various status values
		statuses := []string{"success", "fail", "timeout", "error"}
		for _, status := range statuses {
			assert.NotPanics(t, func() {
				RecordAuthAttempt(ctx, status, duration)
			})
		}
	})
}
