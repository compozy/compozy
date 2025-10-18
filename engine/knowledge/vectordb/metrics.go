//nolint:errcheck // Gauge observations never return errors; wrappers use nil return values intentionally.
package vectordb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	labelUnknownValue = "unknown"
)

// vectorMetric namespace and storage for shared instrumentation state.
var (
	vectorMetricsOnce       sync.Once
	vectorMetricsErr        error
	vectorSearchLatency     metric.Float64Histogram
	vectorResultsCount      metric.Float64Histogram
	vectorMinDistance       metric.Float64Histogram
	vectorActiveConnections metric.Int64ObservableGauge
	vectorErrorsTotal       metric.Int64Counter
	vectorPools             sync.Map
)

type int64Observer interface {
	ObserveInt64(metric.Int64Observable, int64, ...metric.ObserveOption)
}

// ensureVectorMetrics lazily initializes metric instruments used by vector stores.
func ensureVectorMetrics() error {
	vectorMetricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.knowledge.vector")
		if err := initVectorHistograms(meter); err != nil {
			vectorMetricsErr = err
			return
		}
		if err := initVectorCounters(meter); err != nil {
			vectorMetricsErr = err
			return
		}
		if err := initVectorGauge(meter); err != nil {
			vectorMetricsErr = err
		}
	})
	return vectorMetricsErr
}

func initVectorHistograms(meter metric.Meter) error {
	var err error
	vectorSearchLatency, err = meter.Float64Histogram(
		monitoringmetrics.MetricName("vector_similarity_search_seconds"),
		metric.WithDescription("Vector similarity search latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2),
	)
	if err != nil {
		return err
	}
	vectorResultsCount, err = meter.Float64Histogram(
		monitoringmetrics.MetricName("vector_similarity_results_total"),
		metric.WithDescription("Number of results returned per search"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 200),
	)
	if err != nil {
		return err
	}
	vectorMinDistance, err = meter.Float64Histogram(
		monitoringmetrics.MetricName("vector_similarity_distance_min"),
		metric.WithDescription("Minimum distance of top result"),
		metric.WithExplicitBucketBoundaries(0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0),
	)
	return err
}

func initVectorCounters(meter metric.Meter) error {
	var err error
	vectorErrorsTotal, err = meter.Int64Counter(
		monitoringmetrics.MetricName("vector_store_errors_total"),
		metric.WithDescription("Vector store operation errors"),
	)
	return err
}

func initVectorGauge(meter metric.Meter) error {
	var err error
	vectorActiveConnections, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricName("vector_store_connections_active"),
		metric.WithDescription("Active PGVector database connections"),
	)
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		var callbackErr error
		vectorPools.Range(func(key, value any) bool {
			pool, ok := value.(*pgxpool.Pool)
			if !ok || pool == nil {
				return true
			}
			stats := pool.Stat()
			if err := observeInt64Gauge(
				observer,
				vectorActiveConnections,
				int64(stats.AcquiredConns()),
				metric.WithAttributes(attribute.String("vector_db_id", key.(string))),
			); err != nil {
				callbackErr = err
				return false
			}
			return true
		})
		return callbackErr
	}, vectorActiveConnections)
	return err
}

// observeInt64Gauge wraps observer.ObserveInt64 to keep lint noise localized.

func observeInt64Gauge(
	observer metric.Observer,
	observable metric.Int64Observable,
	value int64,
	opts ...metric.ObserveOption,
) error {
	obs, ok := observer.(int64Observer)
	if !ok {
		return fmt.Errorf("observer missing ObserveInt64 support")
	}
	obs.ObserveInt64(observable, value, opts...)
	return nil
}

// recordVectorSearch captures latency, result counts, and distance distribution for similarity queries.
func recordVectorSearch(
	ctx context.Context,
	indexType string,
	topK int,
	duration time.Duration,
	resultCount int,
	minDistance float64,
	includeDistance bool,
) {
	if err := ensureVectorMetrics(); err != nil {
		return
	}
	labels := []attribute.KeyValue{
		attribute.String("index_type", normalizeIndexType(indexType)),
		attribute.Int("top_k", topK),
	}
	vectorSearchLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(labels...))
	vectorResultsCount.Record(ctx, float64(resultCount), metric.WithAttributes(attribute.Int("top_k", topK)))
	if includeDistance && resultCount > 0 {
		vectorMinDistance.Record(
			ctx,
			minDistance,
			metric.WithAttributes(attribute.String("index_type", normalizeIndexType(indexType))),
		)
	}
}

// recordVectorError increments the error counter with normalized labels.
func recordVectorError(ctx context.Context, operation string, errorType string) {
	if err := ensureVectorMetrics(); err != nil || vectorErrorsTotal == nil {
		return
	}
	op := sanitizeLabel(operation, labelUnknownValue)
	errLabel := sanitizeLabel(errorType, labelUnknownValue)
	vectorErrorsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", op),
		attribute.String("error_type", errLabel),
	))
}

// trackVectorPool registers a pgx pool so the gauge callback can observe its statistics.
func trackVectorPool(poolID string, pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	if err := ensureVectorMetrics(); err != nil {
		return
	}
	key := strings.TrimSpace(poolID)
	if key == "" {
		key = labelUnknownValue
	}
	vectorPools.Store(key, pool)
}

func normalizeIndexType(indexType string) string {
	trimmed := strings.TrimSpace(indexType)
	if trimmed == "" {
		return labelUnknownValue
	}
	return strings.ToLower(trimmed)
}

func sanitizeLabel(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return strings.ToLower(trimmed)
}
