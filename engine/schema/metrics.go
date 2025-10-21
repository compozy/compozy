package schema

import (
	"context"
	"fmt"
	"sync"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const schemaMetricSubsystem = "schema"

var (
	schemaMetricsOnce            sync.Once
	schemaMetricsErr             error
	schemaCompileCounter         metric.Int64Counter
	schemaCompileCacheHitCounter metric.Int64Counter
	schemaValidationCounter      metric.Int64Counter
	schemaCompileHistogram       metric.Float64Histogram
	schemaValidateHistogram      metric.Float64Histogram
	schemaCacheGauge             metric.Int64ObservableGauge
	schemaMetricsRegistration    metric.Registration
)

var (
	schemaCompileBuckets  = []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1}
	schemaValidateBuckets = []float64{0.00001, 0.0001, 0.0005, 0.001, 0.005, 0.01}
)

func ensureSchemaMetrics() {
	schemaMetricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.schema")
		schemaMetricsErr = initSchemaMetrics(meter)
	})
}

func initSchemaMetrics(meter metric.Meter) error {
	var err error
	schemaCompileCounter, err = createSchemaCounter(
		meter,
		monitoringmetrics.MetricNameWithSubsystem(schemaMetricSubsystem, "compiles_total"),
		"Total schema compilation attempts",
	)
	if err != nil {
		return err
	}
	schemaCompileCacheHitCounter, err = createSchemaCounter(
		meter,
		monitoringmetrics.MetricNameWithSubsystem(schemaMetricSubsystem, "compile_cache_hits_total"),
		"Schema compilation cache hits",
	)
	if err != nil {
		return err
	}
	schemaValidationCounter, err = createSchemaCounter(
		meter,
		monitoringmetrics.MetricNameWithSubsystem(schemaMetricSubsystem, "validations_total"),
		"Schema validations performed",
	)
	if err != nil {
		return err
	}
	schemaCompileHistogram, err = createSchemaHistogram(
		meter,
		monitoringmetrics.MetricNameWithSubsystem(schemaMetricSubsystem, "compile_duration_seconds"),
		"Schema compilation duration",
		schemaCompileBuckets,
	)
	if err != nil {
		return err
	}
	schemaValidateHistogram, err = createSchemaHistogram(
		meter,
		monitoringmetrics.MetricNameWithSubsystem(schemaMetricSubsystem, "validate_duration_seconds"),
		"Schema validation duration",
		schemaValidateBuckets,
	)
	if err != nil {
		return err
	}
	return registerSchemaGauge(meter)
}

func createSchemaCounter(meter metric.Meter, name string, description string) (metric.Int64Counter, error) {
	return meter.Int64Counter(
		name,
		metric.WithDescription(description),
		metric.WithUnit("1"),
	)
}

func createSchemaHistogram(
	meter metric.Meter,
	name string,
	description string,
	buckets []float64,
) (metric.Float64Histogram, error) {
	return meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(buckets...),
	)
}

func registerSchemaGauge(meter metric.Meter) error {
	gauge, err := meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem(schemaMetricSubsystem, "cache_size"),
		metric.WithDescription("Number of compiled schemas in cache"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	if schemaMetricsRegistration != nil {
		if err := schemaMetricsRegistration.Unregister(); err != nil {
			return fmt.Errorf("schema metrics: unregister callback: %w", err)
		}
	}
	registration, err := meter.RegisterCallback(observeSchemaMetrics, gauge)
	if err != nil {
		return err
	}
	schemaCacheGauge = gauge
	schemaMetricsRegistration = registration
	return nil
}

func observeSchemaMetrics(_ context.Context, observer metric.Observer) error {
	var count int64
	compiledSchemaCache.Range(func(_, _ any) bool {
		count++
		return true
	})
	observer.ObserveInt64(schemaCacheGauge, count)
	return nil
}

func recordSchemaCompile(ctx context.Context, duration time.Duration, cacheHit bool) {
	ensureSchemaMetrics()
	if schemaMetricsErr != nil {
		return
	}
	ctx = metricsContext(ctx)
	if schemaCompileCounter != nil {
		schemaCompileCounter.Add(ctx, 1, metric.WithAttributes(attribute.Bool("cache_hit", cacheHit)))
	}
	if cacheHit && schemaCompileCacheHitCounter != nil {
		schemaCompileCacheHitCounter.Add(ctx, 1)
	}
	if duration > 0 && schemaCompileHistogram != nil {
		schemaCompileHistogram.Record(
			ctx,
			duration.Seconds(),
			metric.WithAttributes(attribute.Bool("cache_hit", cacheHit)),
		)
	}
}

func recordSchemaValidation(ctx context.Context, duration time.Duration, valid bool) {
	ensureSchemaMetrics()
	if schemaMetricsErr != nil {
		return
	}
	ctx = metricsContext(ctx)
	if schemaValidationCounter != nil {
		outcome := "invalid"
		if valid {
			outcome = "valid"
		}
		schemaValidationCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
	}
	if duration > 0 && schemaValidateHistogram != nil {
		schemaValidateHistogram.Record(ctx, duration.Seconds())
	}
}

func metricsContext(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
