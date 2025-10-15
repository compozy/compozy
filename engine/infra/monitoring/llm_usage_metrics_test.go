package monitoring

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestLLMUsageMetrics_RecordSuccess(t *testing.T) {
	t.Helper()
	ctx := t.Context()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, err := newLLMUsageMetrics(meter)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	metrics.RecordSuccess(
		ctx,
		core.ComponentTask,
		"openai",
		"gpt-4o-mini",
		120,
		45,
		50*time.Millisecond,
	)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	var (
		promptFound     bool
		completionFound bool
		eventFound      bool
		latencyFound    bool
	)
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			switch data := metric.Data.(type) {
			case metricdata.Sum[int64]:
				switch metric.Name {
				case llmPromptTokensMetric:
					require.Len(t, data.DataPoints, 1)
					dp := data.DataPoints[0]
					require.Equal(t, int64(120), dp.Value)
					require.Equal(t, string(core.ComponentTask), attrString(t, dp.Attributes, labelComponent))
					require.Equal(t, "openai", attrString(t, dp.Attributes, labelProvider))
					require.Equal(t, "gpt-4o-mini", attrString(t, dp.Attributes, labelModel))
					promptFound = true
				case llmCompletionTokensMetric:
					require.Len(t, data.DataPoints, 1)
					dp := data.DataPoints[0]
					require.Equal(t, int64(45), dp.Value)
					require.Equal(t, string(core.ComponentTask), attrString(t, dp.Attributes, labelComponent))
					completionFound = true
				case llmUsageEventsMetric:
					require.Len(t, data.DataPoints, 1)
					dp := data.DataPoints[0]
					require.Equal(t, int64(1), dp.Value)
					require.Equal(t, outcomeSuccess, attrString(t, dp.Attributes, labelOutcome))
					eventFound = true
				}
			case metricdata.Histogram[float64]:
				if metric.Name != llmUsageLatencyMetric {
					continue
				}
				require.Len(t, data.DataPoints, 1)
				dp := data.DataPoints[0]
				require.Equal(t, uint64(1), dp.Count)
				require.InDelta(t, 0.05, dp.Sum, 1e-6)
				require.Equal(t, outcomeSuccess, attrString(t, dp.Attributes, labelOutcome))
				latencyFound = true
			}
		}
	}
	require.True(t, promptFound, "expected prompt counter datapoint")
	require.True(t, completionFound, "expected completion counter datapoint")
	require.True(t, eventFound, "expected events counter datapoint")
	require.True(t, latencyFound, "expected latency histogram datapoint")
}

func TestLLMUsageMetrics_RecordFailure(t *testing.T) {
	t.Helper()
	ctx := t.Context()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, err := newLLMUsageMetrics(meter)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	metrics.RecordFailure(
		ctx,
		core.ComponentAgent,
		"",
		"",
		75*time.Millisecond,
	)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	var (
		failuresFound bool
		eventFound    bool
		latencyFound  bool
	)
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			switch data := metric.Data.(type) {
			case metricdata.Sum[int64]:
				switch metric.Name {
				case llmUsageFailuresMetric:
					require.Len(t, data.DataPoints, 1)
					dp := data.DataPoints[0]
					require.Equal(t, int64(1), dp.Value)
					require.Equal(t, string(core.ComponentAgent), attrString(t, dp.Attributes, labelComponent))
					failuresFound = true
				case llmUsageEventsMetric:
					require.Len(t, data.DataPoints, 1)
					dp := data.DataPoints[0]
					require.Equal(t, int64(1), dp.Value)
					require.Equal(t, outcomeFailure, attrString(t, dp.Attributes, labelOutcome))
					require.Equal(t, "unknown", attrString(t, dp.Attributes, labelProvider))
					eventFound = true
				}
			case metricdata.Histogram[float64]:
				if metric.Name != llmUsageLatencyMetric {
					continue
				}
				require.Len(t, data.DataPoints, 1)
				dp := data.DataPoints[0]
				require.Equal(t, uint64(1), dp.Count)
				require.InDelta(t, 0.075, dp.Sum, 1e-6)
				require.Equal(t, outcomeFailure, attrString(t, dp.Attributes, labelOutcome))
				latencyFound = true
			}
		}
	}
	require.True(t, failuresFound, "expected failure counter datapoint")
	require.True(t, eventFound, "expected events counter datapoint")
	require.True(t, latencyFound, "expected latency histogram datapoint")
}
