package builtin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestInitMetrics_StepInstruments(t *testing.T) {
	t.Run("ShouldInitializeStepInstruments", func(t *testing.T) {
		ResetMetricsForTesting()
		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		require.NoError(t, err)
		assert.NotNil(t, toolStepExecutionsTotal)
		assert.NotNil(t, toolStepLatencySeconds)
	})
}

func TestRecordStep(t *testing.T) {
	t.Run("ShouldRecordWithoutPanicWhenUninitialized", func(t *testing.T) {
		ResetMetricsForTesting()
		ctx := context.Background()
		assert.NotPanics(t, func() {
			RecordStep(ctx, "cp__test", "agent", "success", 5*time.Millisecond)
		})
	})
	t.Run("ShouldRecordWithoutPanicWhenInitialized", func(t *testing.T) {
		ResetMetricsForTesting()
		ctx := context.Background()
		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		require.NoError(t, err)
		assert.NotPanics(t, func() {
			RecordStep(ctx, "cp__test", "agent", "success", 10*time.Millisecond)
		})
	})
}
