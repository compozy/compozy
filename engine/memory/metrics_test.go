package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestInitMemoryMetrics(t *testing.T) {
	t.Run("Should initialize metrics with valid meter", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset metrics before test
		ResetMemoryMetricsForTesting(ctx)

		// Initialize metrics
		InitMemoryMetrics(ctx, meter)

		// Verify metrics are created
		assert.NotNil(t, memoryMessagesTotal)
		assert.NotNil(t, memoryTokensTotal)
		assert.NotNil(t, memoryOperationLatency)
	})

	t.Run("Should handle nil meter", func(t *testing.T) {
		ctx := context.Background()

		// Reset metrics before test
		ResetMemoryMetricsForTesting(ctx)

		// Initialize with nil meter should not panic
		InitMemoryMetrics(ctx, nil)

		// Metrics should remain nil
		assert.Nil(t, memoryMessagesTotal)
		assert.Nil(t, memoryTokensTotal)
	})

	t.Run("Should only initialize once", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter1 := provider.Meter("test1")
		meter2 := provider.Meter("test2")

		// Reset metrics before test
		ResetMemoryMetricsForTesting(ctx)

		// Initialize first time
		InitMemoryMetrics(ctx, meter1)
		firstCounter := memoryMessagesTotal

		// Initialize second time
		InitMemoryMetrics(ctx, meter2)
		secondCounter := memoryMessagesTotal

		// Should be the same instance
		assert.Equal(t, firstCounter, secondCounter)
	})
}

func TestRecordMemoryMessage(t *testing.T) {
	t.Run("Should record message metrics", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset and initialize metrics
		ResetMemoryMetricsForTesting(ctx)
		InitMemoryMetrics(ctx, meter)

		// Record a message
		RecordMemoryMessage(ctx, "mem-123", "proj-456", 100)

		// Read metrics
		rm := metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify metrics were recorded
		messageMetricFound := false
		tokenMetricFound := false

		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				switch m.Name {
				case "compozy_memory_messages_total":
					messageMetricFound = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Equal(t, int64(1), sum.DataPoints[0].Value)
				case "compozy_memory_tokens_total":
					tokenMetricFound = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Equal(t, int64(100), sum.DataPoints[0].Value)
				}
			}
		}

		assert.True(t, messageMetricFound, "Message metric not found")
		assert.True(t, tokenMetricFound, "Token metric not found")
	})

	t.Run("Should handle nil metrics gracefully", func(_ *testing.T) {
		ctx := context.Background()

		// Reset metrics without initializing
		ResetMemoryMetricsForTesting(ctx)

		// Should not panic
		RecordMemoryMessage(ctx, "mem-123", "proj-456", 100)
	})
}

func TestRecordMemoryOperation(t *testing.T) {
	t.Run("Should record operation latency", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset and initialize metrics
		ResetMemoryMetricsForTesting(ctx)
		InitMemoryMetrics(ctx, meter)

		// Record an operation
		RecordMemoryOperation(ctx, "append", "mem-123", "proj-456", 100*time.Millisecond)

		// Read metrics
		rm := metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify metrics were recorded
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_memory_operation_latency_seconds" {
					found = true
					hist := m.Data.(metricdata.Histogram[float64])
					assert.Equal(t, uint64(1), hist.DataPoints[0].Count)
					assert.InDelta(t, 0.1, hist.DataPoints[0].Sum, 0.01)
				}
			}
		}

		assert.True(t, found, "Operation latency metric not found")
	})
}

func TestGaugeMetrics(t *testing.T) {
	t.Run("Should track goroutine pool state", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset and initialize metrics
		ResetMemoryMetricsForTesting(ctx)
		InitMemoryMetrics(ctx, meter)

		// Update pool state
		UpdateGoroutinePoolState("mem-123", 5, 10)

		// Read metrics
		rm := metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify gauge was recorded
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_memory_goroutine_pool_active" {
					found = true
					gauge := m.Data.(metricdata.Gauge[int64])
					assert.Equal(t, int64(5), gauge.DataPoints[0].Value)
				}
			}
		}

		assert.True(t, found, "Goroutine pool gauge not found")
	})

	t.Run("Should track token usage state", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset and initialize metrics
		ResetMemoryMetricsForTesting(ctx)
		InitMemoryMetrics(ctx, meter)

		// Update token state
		UpdateTokenUsageState("mem-123", 1500, 2000)

		// Read metrics
		rm := metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify gauge was recorded
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_memory_tokens_used" {
					found = true
					gauge := m.Data.(metricdata.Gauge[int64])
					assert.Equal(t, int64(1500), gauge.DataPoints[0].Value)
				}
			}
		}

		assert.True(t, found, "Token usage gauge not found")
	})

	t.Run("Should track health state", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset and initialize metrics
		ResetMemoryMetricsForTesting(ctx)
		InitMemoryMetrics(ctx, meter)

		// Update health state
		UpdateHealthState("mem-123", true, 0)

		// Read metrics
		rm := metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify gauge was recorded
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_memory_health_status" {
					found = true
					gauge := m.Data.(metricdata.Gauge[int64])
					assert.Equal(t, int64(1), gauge.DataPoints[0].Value)
				}
			}
		}

		assert.True(t, found, "Health status gauge not found")

		// Update to unhealthy
		UpdateHealthState("mem-123", false, 3)

		// Read metrics again
		rm = metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify updated state
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_memory_health_status" {
					gauge := m.Data.(metricdata.Gauge[int64])
					assert.Equal(t, int64(0), gauge.DataPoints[0].Value)
				}
			}
		}
	})
}

func TestPrivacyMetrics(t *testing.T) {
	t.Run("Should record privacy exclusions", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset and initialize metrics
		ResetMemoryMetricsForTesting(ctx)
		InitMemoryMetrics(ctx, meter)

		// Record privacy exclusions
		RecordPrivacyExclusion(ctx, "mem-123", "do_not_persist", "proj-456")
		RecordPrivacyExclusion(ctx, "mem-123", "sensitive_data", "proj-456")

		// Read metrics
		rm := metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify metrics were recorded
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_memory_privacy_exclusions_total" {
					found = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Equal(t, int64(2), sum.DataPoints[0].Value+sum.DataPoints[1].Value)
				}
			}
		}

		assert.True(t, found, "Privacy exclusion metric not found")
	})

	t.Run("Should record redaction operations", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Reset and initialize metrics
		ResetMemoryMetricsForTesting(ctx)
		InitMemoryMetrics(ctx, meter)

		// Record redaction
		RecordRedactionOperation(ctx, "mem-123", 3, "proj-456")

		// Read metrics
		rm := metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify metrics were recorded
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_memory_redaction_operations_total" {
					found = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Equal(t, int64(1), sum.DataPoints[0].Value)
				}
			}
		}

		assert.True(t, found, "Redaction operation metric not found")
	})
}

func TestResetMetrics(t *testing.T) {
	t.Run("Should reset all metrics", func(t *testing.T) {
		ctx := context.Background()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Initialize metrics
		InitMemoryMetrics(ctx, meter)

		// Record some data
		RecordMemoryMessage(ctx, "mem-123", "proj-456", 100)
		UpdateHealthState("mem-123", true, 0)

		// Reset metrics
		ResetMemoryMetricsForTesting(ctx)

		// Verify all metrics are nil
		assert.Nil(t, memoryMessagesTotal)
		assert.Nil(t, memoryTokensTotal)
		assert.Nil(t, memoryOperationLatency)
		assert.Nil(t, memoryGoroutinePoolActive)
		assert.Nil(t, memoryTokensUsedGauge)
		assert.Nil(t, memoryHealthStatusGauge)

		// Verify state maps are cleared
		count := 0
		memoryHealthStates.Range(func(_, _ any) bool {
			count++
			return true
		})
		assert.Equal(t, 0, count, "Health states not cleared")
	})
}
