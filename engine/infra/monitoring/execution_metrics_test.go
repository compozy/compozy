package monitoring

import (
	"net/http"
	"testing"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestExecutionMetrics_Recorders(t *testing.T) {
	t.Helper()
	t.Run("ShouldRecordExecutionMetrics", func(t *testing.T) {
		ctx := t.Context()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		metrics, err := newExecutionMetrics(meter)
		require.NoError(t, err)
		require.NotNil(t, metrics)
		metrics.RecordSyncLatency(ctx, ExecutionKindWorkflow, ExecutionOutcomeSuccess, 1500*time.Millisecond)
		metrics.RecordTimeout(ctx, ExecutionKindWorkflow)
		metrics.RecordError(ctx, ExecutionKindWorkflow, http.StatusInternalServerError)
		metrics.RecordAsyncStarted(ctx, ExecutionKindWorkflow)
		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &rm))
		var (
			latencyFound bool
			timeoutFound bool
			errorFound   bool
			asyncFound   bool
		)
		latencyName := monitoringmetrics.MetricNameWithSubsystem("http_exec", "sync_latency_seconds")
		timeoutName := monitoringmetrics.MetricNameWithSubsystem("http_exec", "timeouts_total")
		errorName := monitoringmetrics.MetricNameWithSubsystem("http_exec", "errors_total")
		asyncName := monitoringmetrics.MetricNameWithSubsystem("http_exec", "started_total")
		for _, scopeMetrics := range rm.ScopeMetrics {
			for _, metric := range scopeMetrics.Metrics {
				switch data := metric.Data.(type) {
				case metricdata.Histogram[float64]:
					if metric.Name != latencyName {
						continue
					}
					require.Len(t, data.DataPoints, 1)
					dp := data.DataPoints[0]
					require.Equal(t, uint64(1), dp.Count)
					require.InDelta(t, 1.5, dp.Sum, 0.0001)
					require.Equal(t, ExecutionKindWorkflow, attrString(t, dp.Attributes, "kind"))
					require.Equal(t, ExecutionOutcomeSuccess, attrString(t, dp.Attributes, "outcome"))
					latencyFound = true
				case metricdata.Sum[int64]:
					switch metric.Name {
					case timeoutName:
						require.True(t, data.IsMonotonic)
						require.Equal(t, metricdata.CumulativeTemporality, data.Temporality)
						require.Len(t, data.DataPoints, 1)
						dp := data.DataPoints[0]
						require.Equal(t, int64(1), dp.Value)
						require.Equal(t, ExecutionKindWorkflow, attrString(t, dp.Attributes, "kind"))
						timeoutFound = true
					case errorName:
						require.True(t, data.IsMonotonic)
						require.Equal(t, metricdata.CumulativeTemporality, data.Temporality)
						require.Len(t, data.DataPoints, 1)
						dp := data.DataPoints[0]
						require.Equal(t, int64(1), dp.Value)
						require.Equal(t, ExecutionKindWorkflow, attrString(t, dp.Attributes, "kind"))
						require.Equal(t, int64(http.StatusInternalServerError), attrInt(t, dp.Attributes, "code"))
						errorFound = true
					case asyncName:
						require.True(t, data.IsMonotonic)
						require.Equal(t, metricdata.CumulativeTemporality, data.Temporality)
						require.Len(t, data.DataPoints, 1)
						dp := data.DataPoints[0]
						require.Equal(t, int64(1), dp.Value)
						require.Equal(t, ExecutionKindWorkflow, attrString(t, dp.Attributes, "kind"))
						asyncFound = true
					}
				}
			}
		}
		require.True(t, latencyFound, "expected latency histogram to be collected")
		require.True(t, timeoutFound, "expected timeout counter to be collected")
		require.True(t, errorFound, "expected error counter to be collected")
		require.True(t, asyncFound, "expected async started counter to be collected")
	})
}
