package monitoring

import (
	"testing"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestStreamingMetrics_Instruments(t *testing.T) {
	t.Helper()
	ctx := t.Context()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, err := newStreamingMetrics(meter)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	kind := "workflow"
	metrics.RecordConnect(ctx, kind)
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	requireMetricValue(t, rm, monitoringmetrics.MetricNameWithSubsystem("stream", "active_connections"), kind, 1)

	metrics.RecordEvent(ctx, kind, "workflow_status")
	metrics.RecordTimeToFirstEvent(ctx, kind, 150*time.Millisecond)
	metrics.RecordDuration(ctx, kind, 2*time.Second)
	metrics.RecordError(ctx, kind, "write_failed")
	metrics.RecordDisconnect(ctx, kind)

	rm = metricdata.ResourceMetrics{}
	require.NoError(t, reader.Collect(ctx, &rm))

	assertHistogram(t, rm, monitoringmetrics.MetricNameWithSubsystem("stream", "connection_duration_seconds"), kind, 2)
	assertHistogram(
		t,
		rm,
		monitoringmetrics.MetricNameWithSubsystem("stream", "time_to_first_event_seconds"),
		kind,
		0.15,
	)
	assertCounter(
		t,
		rm,
		monitoringmetrics.MetricNameWithSubsystem("stream", "events_total"),
		kind,
		"event_type",
		"workflow_status",
		1,
	)
	assertCounter(
		t,
		rm,
		monitoringmetrics.MetricNameWithSubsystem("stream", "errors_total"),
		kind,
		"reason",
		"write_failed",
		1,
	)
	requireMetricValue(t, rm, monitoringmetrics.MetricNameWithSubsystem("stream", "active_connections"), kind, 0)
}

func requireMetricValue(t *testing.T, rm metricdata.ResourceMetrics, name, kind string, expected int64) {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok || metric.Name != name {
				continue
			}
			require.Len(t, sum.DataPoints, 1)
			dp := sum.DataPoints[0]
			require.Equal(t, expected, dp.Value)
			require.Equal(t, kind, attrString(t, dp.Attributes, "kind"))
			return
		}
	}
	t.Fatalf("metric %s not found", name)
}

func assertHistogram(t *testing.T, rm metricdata.ResourceMetrics, name, kind string, expectedSum float64) {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			hist, ok := metric.Data.(metricdata.Histogram[float64])
			if !ok || metric.Name != name {
				continue
			}
			require.Len(t, hist.DataPoints, 1)
			dp := hist.DataPoints[0]
			require.Equal(t, kind, attrString(t, dp.Attributes, "kind"))
			require.InDelta(t, expectedSum, dp.Sum, 0.001)
			require.Equal(t, uint64(1), dp.Count)
			return
		}
	}
	t.Fatalf("histogram %s not found", name)
}

func assertCounter(t *testing.T, rm metricdata.ResourceMetrics, name, kind, attrKey, attrValue string, expected int64) {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok || metric.Name != name {
				continue
			}
			require.Len(t, sum.DataPoints, 1)
			dp := sum.DataPoints[0]
			require.Equal(t, expected, dp.Value)
			require.Equal(t, kind, attrString(t, dp.Attributes, "kind"))
			require.Equal(t, attrValue, attrString(t, dp.Attributes, attrKey))
			return
		}
	}
	t.Fatalf("counter %s not found", name)
}
