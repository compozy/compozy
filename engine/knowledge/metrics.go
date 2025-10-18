package knowledge

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	metricsOnce             sync.Once
	metricsMu               sync.Mutex
	metricsInitErr          error
	ingestDurationHist      metric.Float64Histogram
	chunkCounter            metric.Int64Counter
	queryLatencyHist        metric.Float64Histogram
	retrievalAttemptCounter metric.Int64Counter
	retrievalEmptyCounter   metric.Int64Counter
	routerDecisionCounter   metric.Int64Counter
	toolEscalationCounter   metric.Int64Counter
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

func RecordRetrievalAttempt(ctx context.Context, kbID string, stage string) {
	if err := ensureMetrics(); err != nil || retrievalAttemptCounter == nil {
		return
	}
	retrievalAttemptCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("kb_id", kbID),
		attribute.String("stage", stage),
	))
}

func RecordRetrievalEmpty(ctx context.Context, kbID string, stage string) {
	if err := ensureMetrics(); err != nil || retrievalEmptyCounter == nil {
		return
	}
	retrievalEmptyCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("kb_id", kbID),
		attribute.String("stage", stage),
	))
}

func RecordRouterDecision(ctx context.Context, kbID string, decision string) {
	if err := ensureMetrics(); err != nil || routerDecisionCounter == nil {
		return
	}
	routerDecisionCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("kb_id", kbID),
		attribute.String("decision", decision),
	))
}

func RecordToolEscalation(ctx context.Context, kbID string) {
	if err := ensureMetrics(); err != nil || toolEscalationCounter == nil {
		return
	}
	toolEscalationCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("kb_id", kbID)))
}

func ResetMetricsForTesting() {
	metricsMu.Lock()
	metricsOnce = sync.Once{}
	metricsInitErr = nil
	ingestDurationHist = nil
	chunkCounter = nil
	queryLatencyHist = nil
	retrievalAttemptCounter = nil
	retrievalEmptyCounter = nil
	routerDecisionCounter = nil
	toolEscalationCounter = nil
	metricsMu.Unlock()
}

func ensureMetrics() error {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.knowledge")
		if err := initLatencyMetrics(meter); err != nil {
			metricsInitErr = err
			return
		}
		if err := initRetrievalMetrics(meter); err != nil {
			metricsInitErr = err
		}
	})
	return metricsInitErr
}

func initLatencyMetrics(meter metric.Meter) error {
	var err error
	ingestDurationHist, err = meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("knowledge", "ingest_duration_seconds"),
		metric.WithDescription("Latency of knowledge base ingestion runs"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120),
	)
	if err != nil {
		return err
	}
	chunkCounter, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("knowledge", "chunks_total"),
		metric.WithDescription("Number of chunks persisted per knowledge base ingestion"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	queryLatencyHist, err = meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("knowledge", "query_latency_seconds"),
		metric.WithDescription("Latency of knowledge base retrieval queries"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5),
	)
	return err
}

func initRetrievalMetrics(meter metric.Meter) error {
	var err error
	retrievalAttemptCounter, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("knowledge", "retrieval_attempt_total"),
		metric.WithDescription("Number of retrieval attempts performed by stage"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	retrievalEmptyCounter, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("knowledge", "retrieval_empty_total"),
		metric.WithDescription("Number of retrieval attempts that returned no contexts"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	routerDecisionCounter, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("knowledge", "router_decision_total"),
		metric.WithDescription("Number of router decisions classified by outcome"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	toolEscalationCounter, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("knowledge", "tool_escalation_total"),
		metric.WithDescription("Number of times the router escalated to tool usage"),
		metric.WithUnit("1"),
	)
	return err
}
