package ratelimit

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Global metrics for rate limiting
	rateLimitBlocksTotal metric.Int64Counter
	metricsOnce          sync.Once
)

// InitMetrics initializes rate limiting metrics
func InitMetrics(meter metric.Meter) error {
	var err error
	metricsOnce.Do(func() {
		rateLimitBlocksTotal, err = meter.Int64Counter(
			"rate_limit_blocks_total",
			metric.WithDescription("Total number of requests blocked by rate limiting"),
			metric.WithUnit("1"),
		)
	})
	return err
}

// IncrementBlockedRequests increments the rate_limit_blocks_total counter
func IncrementBlockedRequests(ctx context.Context, route string, keyType string) {
	if rateLimitBlocksTotal != nil {
		rateLimitBlocksTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("route", route),
				attribute.String("key_type", keyType),
			),
		)
	}
}
