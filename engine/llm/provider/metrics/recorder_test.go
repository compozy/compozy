package providermetrics

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecorder_RecordsMetrics(t *testing.T) {
	t.Helper()
	ctx := t.Context()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	recorder, err := NewRecorder(meter)
	require.NoError(t, err)
	require.NotNil(t, recorder)

	recorder.RecordRequest(ctx, core.ProviderOpenAI, "gpt-4o", 150*time.Millisecond, "success")
	recorder.RecordTokens(ctx, core.ProviderOpenAI, "gpt-4o", tokenTypePrompt, 120)
	recorder.RecordTokens(ctx, core.ProviderOpenAI, "gpt-4o", tokenTypeOutput, 45)
	recorder.RecordCost(ctx, core.ProviderOpenAI, "gpt-4o", 0.42)
	recorder.RecordError(ctx, core.ProviderOpenAI, "gpt-4o", "rate_limit")
	recorder.RecordRateLimitDelay(ctx, core.ProviderOpenAI, 2*time.Second)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))

	require.NotEmpty(t, rm.ScopeMetrics)
	nameCounts := map[string]int{}
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			nameCounts[metric.Name]++
			switch data := metric.Data.(type) {
			case metricdata.Histogram[float64]:
				require.NotEmpty(t, data.DataPoints)
			case metricdata.Sum[int64]:
				require.NotEmpty(t, data.DataPoints)
			case metricdata.Sum[float64]:
				require.NotEmpty(t, data.DataPoints)
			}
		}
	}
	require.True(t, len(nameCounts) >= 4)
}

func TestRecorder_NopDoesNothing(t *testing.T) {
	rec := Nop()
	rec.RecordRequest(t.Context(), core.ProviderOpenAI, "gpt-4o", time.Second, "success")
	rec.RecordTokens(t.Context(), core.ProviderOpenAI, "gpt-4o", tokenTypePrompt, 10)
	rec.RecordCost(t.Context(), core.ProviderOpenAI, "gpt-4o", 10)
	rec.RecordError(t.Context(), core.ProviderOpenAI, "gpt-4o", "auth")
	rec.RecordRateLimitDelay(t.Context(), core.ProviderOpenAI, time.Second)
}
