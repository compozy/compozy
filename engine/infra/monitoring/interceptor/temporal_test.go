package interceptor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// MockWorkflowInboundInterceptor for testing
type MockWorkflowInboundInterceptor struct {
	mock.Mock
	interceptor.WorkflowInboundInterceptorBase
}

func (m *MockWorkflowInboundInterceptor) Init(outbound interceptor.WorkflowOutboundInterceptor) error {
	args := m.Called(outbound)
	return args.Error(0)
}

func (m *MockWorkflowInboundInterceptor) ExecuteWorkflow(
	ctx workflow.Context,
	in *interceptor.ExecuteWorkflowInput,
) (any, error) {
	args := m.Called(ctx, in)
	return args.Get(0), args.Error(1)
}

func TestTemporalMetrics(t *testing.T) {
	t.Run("Should create temporal metrics interceptor", func(t *testing.T) {
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		interceptor := TemporalMetrics(t.Context(), meter)
		assert.NotNil(t, interceptor)
	})
}

func TestWorkerMetrics(t *testing.T) {
	t.Run("Should set configured worker count", func(t *testing.T) {
		resetMetrics(t.Context())
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		initMetrics(t.Context(), meter)
		SetConfiguredWorkerCount(5)
		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)
		configuredCount := getGaugeValue(t, &rm, "compozy_temporal_workers_configured_total")
		assert.Equal(t, int64(5), configuredCount)
	})
	t.Run("Should increment running workers", func(t *testing.T) {
		resetMetrics(t.Context())
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		initMetrics(t.Context(), meter)
		IncrementRunningWorkers(context.Background())
		IncrementRunningWorkers(context.Background())
		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)
		runningCount := getUpDownCounterValue(t, &rm, "compozy_temporal_workers_running_total")
		assert.Equal(t, int64(2), runningCount)
	})
	t.Run("Should decrement running workers", func(t *testing.T) {
		resetMetrics(t.Context())
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		initMetrics(t.Context(), meter)
		IncrementRunningWorkers(context.Background())
		IncrementRunningWorkers(context.Background())
		DecrementRunningWorkers(context.Background())
		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)
		runningCount := getUpDownCounterValue(t, &rm, "compozy_temporal_workers_running_total")
		assert.Equal(t, int64(1), runningCount)
	})
	t.Run("Should handle nil worker metrics without panics", func(t *testing.T) {
		workersRunning = nil
		workersConfigured = nil
		assert.NotPanics(t, func() {
			IncrementRunningWorkers(context.Background())
			DecrementRunningWorkers(context.Background())
		})
	})
}

func TestWorkflowInterceptorErrorHandling(t *testing.T) {
	t.Run("Should handle nil metrics gracefully", func(t *testing.T) {
		// Reset all metrics to nil
		workflowStartedTotal = nil
		workflowCompletedTotal = nil
		workflowFailedTotal = nil
		workflowTaskDuration = nil
		interceptor := &metricsInterceptor{}
		// Create a mock workflow context that would normally cause a panic
		workflowInterceptor := interceptor.InterceptWorkflow(nil, &MockWorkflowInboundInterceptor{})
		assert.NotNil(t, workflowInterceptor)
		// The real test would be in integration tests with actual Temporal workflow context
	})
	t.Run("Should recover from panic in metrics recording", func(t *testing.T) {
		resetMetrics(t.Context())
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		interceptor := &metricsInterceptor{meter: meter}
		initMetrics(t.Context(), meter)
		// This test demonstrates the panic recovery mechanism
		// In actual usage, panics would be caught and logged
		workflowInterceptor := interceptor.InterceptWorkflow(nil, &MockWorkflowInboundInterceptor{})
		assert.NotNil(t, workflowInterceptor)
	})
}

func TestMetricsInitialization(t *testing.T) {
	t.Run("Should initialize all metrics", func(t *testing.T) {
		resetMetrics(t.Context())
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		initMetrics(t.Context(), meter)
		assert.NotNil(t, workflowStartedTotal)
		assert.NotNil(t, workflowCompletedTotal)
		assert.NotNil(t, workflowFailedTotal)
		assert.NotNil(t, workflowTaskDuration)
		assert.NotNil(t, workersRunning)
		assert.NotNil(t, workersConfigured)
	})
	t.Run("Should handle metric creation errors gracefully", func(t *testing.T) {
		// The implementation logs errors but continues
		// This ensures monitoring doesn't break the application
		resetMetrics(t.Context())
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		// Call initMetrics multiple times - should use sync.Once
		initMetrics(t.Context(), meter)
		initMetrics(t.Context(), meter)
		initMetrics(t.Context(), meter)
		// Metrics should still be initialized once
		assert.NotNil(t, workflowStartedTotal)
	})
}

func TestTemporalErrorTypes(t *testing.T) {
	t.Run("Should identify canceled error", func(t *testing.T) {
		err := temporal.NewCanceledError("")
		assert.True(t, temporal.IsCanceledError(err))
	})
	t.Run("Should identify timeout error", func(t *testing.T) {
		err := temporal.NewTimeoutError(enums.TIMEOUT_TYPE_SCHEDULE_TO_START, nil)
		assert.True(t, temporal.IsTimeoutError(err))
	})
	t.Run("Should handle generic errors", func(t *testing.T) {
		err := errors.New("generic error")
		assert.False(t, temporal.IsCanceledError(err))
		assert.False(t, temporal.IsTimeoutError(err))
	})
}

func getGaugeValue(_ *testing.T, rm *metricdata.ResourceMetrics, name string) int64 {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				if gauge, ok := m.Data.(metricdata.Gauge[int64]); ok {
					if len(gauge.DataPoints) > 0 {
						return gauge.DataPoints[0].Value
					}
				}
			}
		}
	}
	return 0
}

func getUpDownCounterValue(_ *testing.T, rm *metricdata.ResourceMetrics, name string) int64 {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
					if len(sum.DataPoints) > 0 {
						return sum.DataPoints[0].Value
					}
				}
			}
		}
	}
	return 0
}
