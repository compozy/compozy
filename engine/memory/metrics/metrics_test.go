package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestInitMemoryMetrics(t *testing.T) {
	t.Run("Should initialize all metrics with valid meter", func(t *testing.T) {
		// Reset metrics before test to ensure clean state
		ResetMemoryMetricsForTesting(t.Context())
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")

		// Initialize metrics
		InitMemoryMetrics(ctx, meter)

		// Verify all counter metrics are initialized
		assert.NotNil(t, memoryMessagesTotal, "memoryMessagesTotal should be initialized")
		assert.NotNil(t, memoryTokensTotal, "memoryTokensTotal should be initialized")
		assert.NotNil(t, memoryTrimTotal, "memoryTrimTotal should be initialized")
		assert.NotNil(t, memoryFlushTotal, "memoryFlushTotal should be initialized")
		assert.NotNil(t, memoryLockAcquireTotal, "memoryLockAcquireTotal should be initialized")
		assert.NotNil(t, memoryLockContentionTotal, "memoryLockContentionTotal should be initialized")
		assert.NotNil(t, memoryTokensSavedTotal, "memoryTokensSavedTotal should be initialized")
		assert.NotNil(t, memoryRedactionOperations, "memoryRedactionOperations should be initialized")
		assert.NotNil(t, memoryCircuitBreakerTrips, "memoryCircuitBreakerTrips should be initialized")

		// Verify histogram metrics are initialized
		assert.NotNil(t, memoryOperationLatency, "memoryOperationLatency should be initialized")

		// Verify gauge metrics are initialized
		assert.NotNil(t, memoryGoroutinePoolActive, "memoryGoroutinePoolActive should be initialized")
		assert.NotNil(t, memoryTokensUsedGauge, "memoryTokensUsedGauge should be initialized")
		assert.NotNil(t, memoryHealthStatusGauge, "memoryHealthStatusGauge should be initialized")
	})

	t.Run("Should handle nil meter gracefully", func(t *testing.T) {
		ResetMemoryMetricsForTesting(t.Context())
		ctx := t.Context()

		// Should not panic with nil meter
		assert.NotPanics(t, func() {
			InitMemoryMetrics(ctx, nil)
		})
	})

	t.Run("Should only initialize once with sync.Once", func(t *testing.T) {
		ResetMemoryMetricsForTesting(t.Context())
		ctx := t.Context()
		meter1 := noop.NewMeterProvider().Meter("test1")
		meter2 := noop.NewMeterProvider().Meter("test2")

		// First initialization
		InitMemoryMetrics(ctx, meter1)
		assert.NotNil(t, memoryMessagesTotal)

		// Second initialization should be ignored due to sync.Once
		InitMemoryMetrics(ctx, meter2)
		// Metrics should still be from first initialization
		assert.NotNil(t, memoryMessagesTotal)
	})
}

func TestRecordMemoryMessage(t *testing.T) {
	t.Run("Should record memory message with valid parameters", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		// Verify metrics are initialized
		require.NotNil(t, memoryMessagesTotal)
		require.NotNil(t, memoryTokensTotal)

		// Should not panic and should handle recording
		assert.NotPanics(t, func() {
			RecordMemoryMessage(ctx, "test-memory-1", "test-project-1", 50)
		})
	})

	t.Run("Should handle zero tokens gracefully", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		// Should not panic with zero tokens
		assert.NotPanics(t, func() {
			RecordMemoryMessage(ctx, "test-memory-2", "test-project-2", 0)
		})
	})

	t.Run("Should handle nil metrics gracefully", func(t *testing.T) {
		ctx := t.Context()
		// Reset to ensure nil state
		ResetMemoryMetricsForTesting(t.Context())

		// Should not panic when metrics are nil
		assert.NotPanics(t, func() {
			RecordMemoryMessage(ctx, "test-memory-3", "test-project-3", 100)
		})
	})
}

func TestRecordMemoryTrim(t *testing.T) {
	t.Run("Should record memory trim with valid parameters", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		// Verify metrics are initialized
		require.NotNil(t, memoryTrimTotal)
		require.NotNil(t, memoryTokensSavedTotal)

		// Should not panic and should handle recording
		assert.NotPanics(t, func() {
			RecordMemoryTrim(ctx, "test-memory-1", "test-project-1", "fifo", 500)
		})
	})

	t.Run("Should handle different trim strategies", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		strategies := []string{"fifo", "lifo", "importance", "hybrid"}
		for _, strategy := range strategies {
			assert.NotPanics(t, func() {
				RecordMemoryTrim(ctx, "test-memory", "test-project", strategy, 100)
			})
		}
	})

	t.Run("Should handle zero tokens saved", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		// Should not panic with zero tokens saved
		assert.NotPanics(t, func() {
			RecordMemoryTrim(ctx, "test-memory", "test-project", "fifo", 0)
		})
	})
}

func TestRecordMemoryFlush(t *testing.T) {
	t.Run("Should record memory flush with valid parameters", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		require.NotNil(t, memoryFlushTotal)

		// Test different flush types
		flushTypes := []string{"manual", "automatic", "size_limit", "time_based"}
		for _, flushType := range flushTypes {
			assert.NotPanics(t, func() {
				RecordMemoryFlush(ctx, "test-memory", "test-project", flushType)
			})
		}
	})
}

func TestRecordMemoryLockOperations(t *testing.T) {
	t.Run("Should record lock acquisition", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		require.NotNil(t, memoryLockAcquireTotal)

		assert.NotPanics(t, func() {
			RecordMemoryLockAcquire(ctx, "test-memory", "test-project")
		})
	})

	t.Run("Should record lock contention", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		require.NotNil(t, memoryLockContentionTotal)

		assert.NotPanics(t, func() {
			RecordMemoryLockContention(ctx, "test-memory", "test-project")
		})
	})
}

func TestRecordMemoryOp(t *testing.T) {
	t.Run("Should record memory operation with latency", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		require.NotNil(t, memoryOperationLatency)

		// Test recording with latency
		assert.NotPanics(t, func() {
			RecordMemoryOp(ctx, "test-memory", "test-project", "add_message", 10*time.Millisecond, 100, nil)
		})
	})

	t.Run("Should handle operation errors", func(t *testing.T) {
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		// Should not panic even when operation returns error
		assert.NotPanics(t, func() {
			RecordMemoryOp(ctx, "test-memory", "test-project", "failing_op", 5*time.Millisecond, 50, assert.AnError)
		})
	})
}

func TestStateManagement(t *testing.T) {
	metricsState := GetDefaultState()

	t.Run("Should update goroutine pool state", func(t *testing.T) {
		memoryID := "test-memory-pool"
		metricsState.UpdatePoolState(memoryID, 5, 10)

		// Verify state was created
		poolState, exists := metricsState.GetPoolState(memoryID)
		assert.True(t, exists)
		assert.Equal(t, int64(5), poolState.ActiveCount)
		assert.Equal(t, int64(10), poolState.MaxPoolSize)

		// Update again and verify
		metricsState.UpdatePoolState(memoryID, 10, 50)
		poolState, _ = metricsState.GetPoolState(memoryID)
		assert.Equal(t, int64(10), poolState.ActiveCount)
		assert.Equal(t, int64(50), poolState.MaxPoolSize)
	})

	t.Run("Should update token usage state", func(t *testing.T) {
		memoryID := "test-memory-tokens"
		metricsState.UpdateTokenState(memoryID, 1000, 5000)

		// Verify state was created
		tokenState, exists := metricsState.GetTokenState(memoryID)
		assert.True(t, exists)
		assert.Equal(t, memoryID, tokenState.MemoryID)
		assert.Equal(t, int64(1000), tokenState.TokensUsed)
		assert.Equal(t, int64(5000), tokenState.MaxTokens)

		// Update again and verify
		metricsState.UpdateTokenState(memoryID, 2000, 5000)
		tokenState, _ = metricsState.GetTokenState(memoryID)
		assert.Equal(t, int64(2000), tokenState.TokensUsed)
	})

	t.Run("Should update health state", func(t *testing.T) {
		memoryID := "test-memory-health"
		metricsState.UpdateHealthState(memoryID, true, 0)

		// Verify state was created
		healthState, exists := metricsState.GetHealthState(memoryID)
		assert.True(t, exists)
		assert.Equal(t, memoryID, healthState.MemoryID)
		assert.True(t, healthState.IsHealthy)
		assert.Equal(t, 0, healthState.ConsecutiveFailures)
		assert.WithinDuration(t, time.Now(), healthState.LastHealthCheck, 1*time.Second)

		// Update to unhealthy state
		metricsState.UpdateHealthState(memoryID, false, 3)
		healthState, _ = metricsState.GetHealthState(memoryID)
		assert.False(t, healthState.IsHealthy)
		assert.Equal(t, 3, healthState.ConsecutiveFailures)
	})
}

func TestResetMetrics(t *testing.T) {
	t.Run("Should reset all metrics to nil", func(t *testing.T) {
		// First initialize
		ctx := t.Context()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)

		// Verify initialized
		assert.NotNil(t, memoryMessagesTotal)

		// Reset
		ResetMemoryMetricsForTesting(t.Context())

		// Verify reset
		assert.Nil(t, memoryMessagesTotal)
		assert.Nil(t, memoryTokensTotal)
		assert.Nil(t, memoryTrimTotal)
		assert.Nil(t, memoryFlushTotal)
		assert.Nil(t, memoryRedactionOperations)
		assert.Nil(t, memoryCircuitBreakerTrips)
		assert.Nil(t, memoryOperationLatency)
		assert.Nil(t, memoryGoroutinePoolActive)
		assert.Nil(t, memoryTokensUsedGauge)
		assert.Nil(t, memoryHealthStatusGauge)
	})
}
