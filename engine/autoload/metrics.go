package autoload

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	autoloadMetricSubsystem = "autoload"

	projectLabelUnknown = "unknown"
)

var (
	autoloadDurationBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30}

	supportedConfigMetricTypes = map[string]struct{}{
		"workflow": {},
		"agent":    {},
		"tool":     {},
		"mcp":      {},
	}
)

type autoloadFileOutcome string

const (
	autoloadOutcomeSuccess autoloadFileOutcome = "success"
	autoloadOutcomeError   autoloadFileOutcome = "error"
)

type autoloadErrorLabel string

const (
	errorLabelParse      autoloadErrorLabel = "parse_error"
	errorLabelValidation autoloadErrorLabel = "validation_error"
	errorLabelDuplicate  autoloadErrorLabel = "duplicate_error"
	errorLabelSecurity   autoloadErrorLabel = "security_error"
)

type autoloadMetrics struct {
	initOnce sync.Once

	durationHistogram metric.Float64Histogram
	filesProcessed    metric.Int64Counter
	configsLoaded     metric.Int64Counter
	errorsTotal       metric.Int64Counter
}

var metricsContainer autoloadMetrics

func autoloadMetricsRecorder() *autoloadMetrics {
	metricsContainer.initOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.autoload")
		var err error

		metricsContainer.durationHistogram, err = meter.Float64Histogram(
			monitoringmetrics.MetricNameWithSubsystem(autoloadMetricSubsystem, "duration_seconds"),
			metric.WithDescription("Time to complete the autoload process"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(autoloadDurationBuckets...),
		)
		if err != nil {
			panic(fmt.Errorf("autoload metrics: duration histogram: %w", err))
		}

		metricsContainer.filesProcessed, err = meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem(autoloadMetricSubsystem, "files_processed_total"),
			metric.WithDescription("Total files processed by autoload"),
			metric.WithUnit("1"),
		)
		if err != nil {
			panic(fmt.Errorf("autoload metrics: files processed counter: %w", err))
		}

		metricsContainer.configsLoaded, err = meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem(autoloadMetricSubsystem, "configs_loaded_total"),
			metric.WithDescription("Total configurations loaded by type"),
			metric.WithUnit("1"),
		)
		if err != nil {
			panic(fmt.Errorf("autoload metrics: configs loaded counter: %w", err))
		}

		metricsContainer.errorsTotal, err = meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem(autoloadMetricSubsystem, "errors_total"),
			metric.WithDescription("Total autoload errors by category"),
			metric.WithUnit("1"),
		)
		if err != nil {
			panic(fmt.Errorf("autoload metrics: errors counter: %w", err))
		}
	})
	return &metricsContainer
}

func recordAutoloadDuration(ctx context.Context, project string, duration time.Duration) {
	recorder := autoloadMetricsRecorder()
	if recorder.durationHistogram == nil {
		return
	}
	recorder.durationHistogram.Record(ctx, duration.Seconds(),
		metric.WithAttributes(attribute.String("project", sanitizeProjectLabel(project))),
	)
}

func recordAutoloadFileOutcome(ctx context.Context, project string, outcome autoloadFileOutcome) {
	recorder := autoloadMetricsRecorder()
	if recorder.filesProcessed == nil {
		return
	}
	recorder.filesProcessed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("project", sanitizeProjectLabel(project)),
			attribute.String("outcome", string(outcome)),
		),
	)
}

func recordAutoloadConfigLoaded(ctx context.Context, project string, normalizedResourceType string) {
	if normalizedResourceType == "" {
		return
	}
	if _, ok := supportedConfigMetricTypes[normalizedResourceType]; !ok {
		return
	}
	recorder := autoloadMetricsRecorder()
	if recorder.configsLoaded == nil {
		return
	}
	recorder.configsLoaded.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("project", sanitizeProjectLabel(project)),
			attribute.String("type", normalizedResourceType),
		),
	)
}

func recordAutoloadError(ctx context.Context, project string, label autoloadErrorLabel) {
	recorder := autoloadMetricsRecorder()
	if recorder.errorsTotal == nil {
		return
	}
	recorder.errorsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("project", sanitizeProjectLabel(project)),
			attribute.String("error_type", string(label)),
		),
	)
}

func sanitizeProjectLabel(project string) string {
	trimmed := strings.TrimSpace(project)
	if trimmed == "" {
		return projectLabelUnknown
	}
	return trimmed
}
