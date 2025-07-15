package auth

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Global metrics for auth
	authRequestsTotal metric.Int64Counter
	authLatency       metric.Float64Histogram
	metricsOnce       sync.Once
	metricsMutex      sync.Mutex
)

// InitMetrics initializes auth metrics
func InitMetrics(meter metric.Meter) error {
	if meter == nil {
		return nil
	}

	var err error
	metricsOnce.Do(func() {
		authRequestsTotal, err = meter.Int64Counter(
			"auth_requests_total",
			metric.WithDescription("Total number of auth requests"),
			metric.WithUnit("1"),
		)
		if err != nil {
			return
		}

		authLatency, err = meter.Float64Histogram(
			"auth_latency_seconds",
			metric.WithDescription("Auth middleware latency"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
		)
	})
	return err
}

// ResetMetricsForTesting resets the metrics initialization state for testing.
// This function should only be used in tests to ensure clean state between test runs.
func ResetMetricsForTesting() {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	authRequestsTotal = nil
	authLatency = nil
	metricsOnce = sync.Once{}
}

// RecordAuthAttempt records auth request metrics
func RecordAuthAttempt(ctx context.Context, status string, duration time.Duration) {
	if authRequestsTotal != nil {
		authRequestsTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("status", status),
			),
		)
	}
	if authLatency != nil {
		authLatency.Record(ctx, duration.Seconds())
	}
}
