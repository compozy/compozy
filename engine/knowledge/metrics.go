package knowledge

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	metricsOnce        sync.Once
	metricsMu          sync.Mutex
	metricsInitErr     error
	ingestDurationHist metric.Float64Histogram
	chunkCounter       metric.Int64Counter
	queryLatencyHist   metric.Float64Histogram
)

func RecordIngestDuration(ctx context.Context, kbID string, d time.Duration) {
	if err := ensureMetrics(); err != nil || ingestDurationHist == nil {
		return
	}
	ingestDurationHist.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String("kb_id", kbID)))
}

func RecordIngestChunks(ctx context.Context, kbID string, chunks int) {
	if chunks <= 0 {
		return
	}
	if err := ensureMetrics(); err != nil || chunkCounter == nil {
		return
	}
	chunkCounter.Add(ctx, int64(chunks), metric.WithAttributes(attribute.String("kb_id", kbID)))
}

func RecordQueryLatency(ctx context.Context, kbID string, d time.Duration) {
	if err := ensureMetrics(); err != nil || queryLatencyHist == nil {
		return
	}
	queryLatencyHist.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String("kb_id", kbID)))
}

func ResetMetricsForTesting() {
	metricsMu.Lock()
	metricsOnce = sync.Once{}
	metricsInitErr = nil
	ingestDurationHist = nil
	chunkCounter = nil
	queryLatencyHist = nil
	metricsMu.Unlock()
}

func ensureMetrics() error {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.knowledge")
		var err error
		ingestDurationHist, err = meter.Float64Histogram(
			"knowledge_ingest_duration_seconds",
			metric.WithDescription("Latency of knowledge base ingestion runs"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(.05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120),
		)
		if err != nil {
			metricsInitErr = err
			return
		}
		chunkCounter, err = meter.Int64Counter(
			"knowledge_chunks_total",
			metric.WithDescription("Number of chunks persisted per knowledge base ingestion"),
			metric.WithUnit("1"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}
		queryLatencyHist, err = meter.Float64Histogram(
			"knowledge_query_latency_seconds",
			metric.WithDescription("Latency of knowledge base retrieval queries"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5),
		)
		if err != nil {
			metricsInitErr = err
		}
	})
	return metricsInitErr
}
