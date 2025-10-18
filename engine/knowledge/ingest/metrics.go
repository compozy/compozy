package ingest

import (
	"context"
	"strings"
	"sync"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	StageChunking  = "chunking"
	StageEmbedding = "embedding"
	StageStorage   = "storage"

	OutcomeSuccess = "success"
	OutcomeError   = "error"
)

const (
	stageUnknownValue = "unknown"

	errorTypeInternal  = "internal"
	errorTypeRateLimit = "rate_limit"
	errorTypeAuth      = "auth"
	errorTypeInvalid   = "invalid"
	errorTypeTimeout   = "timeout"
	errorTypeCanceled  = "canceled"
)

var (
	metricsOnce        sync.Once
	metricsMu          sync.Mutex
	metricsInitErr     error
	pipelineLatency    metric.Float64Histogram
	documentsCounter   metric.Int64Counter
	chunksCounter      metric.Int64Counter
	batchSizeHistogram metric.Float64Histogram
	errorsCounter      metric.Int64Counter
)

// RecordPipelineStage captures the latency for a specific ingestion stage.
func RecordPipelineStage(ctx context.Context, stage string, duration time.Duration) {
	if duration <= 0 {
		return
	}
	if err := ensureMetrics(); err != nil || pipelineLatency == nil {
		return
	}
	pipelineLatency.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributes(attribute.String("stage", normalizeStage(stage))),
	)
}

// RecordDocuments increments the total number of documents processed by outcome.
func RecordDocuments(ctx context.Context, count int, outcome string) {
	if count <= 0 {
		return
	}
	if err := ensureMetrics(); err != nil || documentsCounter == nil {
		return
	}
	documentsCounter.Add(
		ctx,
		int64(count),
		metric.WithAttributes(attribute.String("outcome", normalizeOutcome(outcome))),
	)
}

// RecordChunks increments the total number of chunks generated during ingestion.
func RecordChunks(ctx context.Context, count int) {
	if count <= 0 {
		return
	}
	if err := ensureMetrics(); err != nil || chunksCounter == nil {
		return
	}
	chunksCounter.Add(ctx, int64(count))
}

// RecordBatchSize observes the number of documents processed in a batch run.
func RecordBatchSize(ctx context.Context, size int) {
	if size <= 0 {
		return
	}
	if err := ensureMetrics(); err != nil || batchSizeHistogram == nil {
		return
	}
	batchSizeHistogram.Record(ctx, float64(size))
}

// RecordError counts pipeline errors annotated with stage and error type.
func RecordError(ctx context.Context, stage string, errorType string) {
	if err := ensureMetrics(); err != nil || errorsCounter == nil {
		return
	}
	errorsCounter.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("stage", normalizeStage(stage)),
			attribute.String("error_type", normalizeErrorType(errorType)),
		),
	)
}

// ResetMetricsForTesting clears metric state to allow deterministic test assertions.
func ResetMetricsForTesting() {
	metricsMu.Lock()
	metricsOnce = sync.Once{}
	metricsInitErr = nil
	pipelineLatency = nil
	documentsCounter = nil
	chunksCounter = nil
	batchSizeHistogram = nil
	errorsCounter = nil
	metricsMu.Unlock()
}

func ensureMetrics() error {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.knowledge.ingest")
		metricsInitErr = initMetrics(meter)
	})
	return metricsInitErr
}

func initMetrics(meter metric.Meter) error {
	var err error
	pipelineLatency, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("knowledge_ingestion", "pipeline_seconds"),
		metric.WithDescription("Ingestion pipeline stage duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.1, .5, 1, 2, 5, 10, 30, 60, 120),
	)
	if err != nil {
		return err
	}
	documentsCounter, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("knowledge_ingestion", "documents_total"),
		metric.WithDescription("Documents processed through ingestion"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	chunksCounter, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("knowledge_ingestion", "chunks_total"),
		metric.WithDescription("Total chunks generated from documents"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	batchSizeHistogram, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("knowledge_ingestion", "batch_size"),
		metric.WithDescription("Documents per ingestion batch"),
		metric.WithUnit("1"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 200),
	)
	if err != nil {
		return err
	}
	errorsCounter, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("knowledge_ingestion", "errors_total"),
		metric.WithDescription("Ingestion errors by stage"),
		metric.WithUnit("1"),
	)
	return err
}

func normalizeStage(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case StageChunking:
		return StageChunking
	case StageEmbedding:
		return StageEmbedding
	case StageStorage:
		return StageStorage
	default:
		return stageUnknownValue
	}
}

func normalizeOutcome(outcome string) string {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case OutcomeSuccess:
		return OutcomeSuccess
	case OutcomeError:
		return OutcomeError
	default:
		return OutcomeError
	}
}

func normalizeErrorType(errType string) string {
	clean := strings.TrimSpace(errType)
	if clean == "" {
		return errorTypeInternal
	}
	return clean
}
