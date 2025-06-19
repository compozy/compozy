package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewMetrics(t *testing.T) {
	t.Run("Should create metrics with valid meter", func(t *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")

		// Act
		metrics := NewMetrics(ctx, meter)

		// Assert
		assert.NotNil(t, metrics)
		assert.Equal(t, meter, metrics.meter)
		assert.NotNil(t, metrics.log)
	})

	t.Run("Should handle nil meter gracefully", func(t *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())

		// Act
		metrics := NewMetrics(ctx, nil)

		// Assert
		assert.NotNil(t, metrics)
		assert.Nil(t, metrics.meter)
	})
}

func TestScheduleMetrics_RecordOperation(t *testing.T) {
	t.Run("Should record operation with valid meter", func(_ *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")
		metrics := NewMetrics(ctx, meter)

		// Act & Assert - Should not panic
		metrics.RecordOperation(ctx, "create", "success", "test-project")
	})

	t.Run("Should handle nil instruments gracefully", func(_ *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		metrics := NewMetrics(ctx, nil)

		// Act & Assert - Should not panic
		metrics.RecordOperation(ctx, "create", "success", "test-project")
	})
}

func TestScheduleMetrics_UpdateWorkflowCount(t *testing.T) {
	t.Run("Should update workflow count", func(_ *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")
		metrics := NewMetrics(ctx, meter)

		// Act & Assert - Should not panic
		metrics.UpdateWorkflowCount(ctx, "test-project", "active", 5)
		metrics.UpdateWorkflowCount(ctx, "test-project", "paused", 2)
	})
}

func TestScheduleMetrics_RecordReconcileDuration(t *testing.T) {
	t.Run("Should record reconciliation duration", func(_ *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")
		metrics := NewMetrics(ctx, meter)
		duration := 2 * time.Second

		// Act & Assert - Should not panic
		metrics.RecordReconcileDuration(ctx, "test-project", duration)
	})
}

func TestScheduleMetrics_StartEndReconciliation(t *testing.T) {
	t.Run("Should track reconciliation lifecycle", func(_ *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")
		metrics := NewMetrics(ctx, meter)

		// Act & Assert - Should not panic
		metrics.StartReconciliation(ctx, "test-project")
		metrics.EndReconciliation(ctx, "test-project")
	})
}

func TestReconciliationTracker(t *testing.T) {
	t.Run("Should track reconciliation lifecycle", func(t *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")
		metrics := NewMetrics(ctx, meter)

		// Act
		tracker := metrics.NewReconciliationTracker(ctx, "test-project")
		require.NotNil(t, tracker)
		assert.Equal(t, metrics, tracker.metrics)
		assert.Equal(t, "test-project", tracker.project)
		assert.False(t, tracker.startTime.IsZero())

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		// Finish tracking
		tracker.Finish()

		// Should complete without panic
	})

	t.Run("Should handle tracker with nil meter", func(_ *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		metrics := NewMetrics(ctx, nil)

		// Act
		tracker := metrics.NewReconciliationTracker(ctx, "test-project")
		tracker.Finish()

		// Should complete without panic
	})
}

func TestResetMetricsForTesting(t *testing.T) {
	t.Run("Should reset metrics state", func(t *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")

		// Initialize metrics
		_ = NewMetrics(ctx, meter)

		// Act
		ResetMetricsForTesting()

		// Assert - Should be able to initialize again
		metrics := NewMetrics(ctx, meter)
		assert.NotNil(t, metrics)
	})
}

func TestMetricsInitialization(t *testing.T) {
	// Reset state before test
	ResetMetricsForTesting()
	defer ResetMetricsForTesting()

	t.Run("Should initialize metrics only once", func(t *testing.T) {
		// Arrange
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		meter := noop.NewMeterProvider().Meter("test")

		// Act - Create multiple instances
		metrics1 := NewMetrics(ctx, meter)
		metrics2 := NewMetrics(ctx, meter)

		// Assert
		assert.NotNil(t, metrics1)
		assert.NotNil(t, metrics2)
		// Both should work without issues
		metrics1.RecordOperation(ctx, "create", "success", "project1")
		metrics2.RecordOperation(ctx, "update", "failure", "project2")
	})
}
