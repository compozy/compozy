package embeddings

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestRecordFunctionsDoNotPanic(t *testing.T) {
	ctx := context.Background()

	resetMetrics()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(noop.NewMeterProvider())
	t.Cleanup(func() {
		otel.SetMeterProvider(prev)
		resetMetrics()
	})

	assert.NotPanics(t, func() {
		RecordCacheHit(ctx, "openai")
		RecordCacheMiss(ctx, "openai")
		RecordGeneration(ctx, "openai", "text-embedding-3-small", 1, 0, 0)
		RecordError(ctx, "openai", "text-embedding-3-small", ErrorTypeServerError)
	})
}

func TestEstimateTokens(t *testing.T) {
	ctx := context.Background()
	resetTokenCounters()
	tokens, err := EstimateTokens(ctx, "openai", "text-embedding-3-small", []string{"hello world", "another sample"})
	require.NoError(t, err)
	assert.Greater(t, tokens, 0)

	// Ensure cached counter reuse does not fail
	repeated, err := EstimateTokens(ctx, "openai", "text-embedding-3-small", []string{"hello"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, repeated, 1)
}
