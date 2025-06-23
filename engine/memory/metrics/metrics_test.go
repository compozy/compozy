package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestInitMemoryMetrics(t *testing.T) {
	t.Run("Should initialize metrics with valid meter", func(_ *testing.T) {
		ctx := context.Background()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)
		// Should complete without error
	})
}

func TestRecordMemoryMessage(t *testing.T) {
	t.Run("Should record memory message without error", func(_ *testing.T) {
		// Initialize with noop meter to avoid actual metrics recording
		ctx := context.Background()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)
		// Should not panic or error
		RecordMemoryMessage(ctx, "test-memory", "test-project", 50)
	})
}

func TestRecordMemoryTrim(t *testing.T) {
	t.Run("Should record memory trim", func(_ *testing.T) {
		ctx := context.Background()
		meter := noop.NewMeterProvider().Meter("test")
		InitMemoryMetrics(ctx, meter)
		RecordMemoryTrim(ctx, "test-memory", "test-project", "fifo", 500)
		// Should complete without error
	})
}

func TestMetricsState(t *testing.T) {
	t.Run("Should handle metrics state maps", func(t *testing.T) {
		// Test that state maps exist and can be used
		MemoryPoolStates.Store("test", &struct{}{})
		MemoryTokenStates.Store("test", &struct{}{})
		MemoryHealthStates.Store("test", &struct{}{})
		_, exists := MemoryPoolStates.Load("test")
		assert.True(t, exists)
		_, exists = MemoryTokenStates.Load("test")
		assert.True(t, exists)
		_, exists = MemoryHealthStates.Load("test")
		assert.True(t, exists)
	})
}
