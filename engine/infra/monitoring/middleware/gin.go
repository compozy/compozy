package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
)

var (
	httpRequestsTotal    metric.Int64Counter
	httpRequestDuration  metric.Float64Histogram
	httpRequestsInFlight metric.Int64UpDownCounter
	initOnce             sync.Once
	initMutex            sync.Mutex
)

// initMetrics initializes the HTTP metrics instruments
func initMetrics(ctx context.Context, meter metric.Meter) {
	// Skip initialization if meter is nil
	if meter == nil {
		return
	}
	log := logger.FromContext(ctx)
	initOnce.Do(func() {
		var err error
		httpRequestsTotal, err = meter.Int64Counter(
			"compozy_http_requests",
			metric.WithDescription("Total HTTP requests"),
		)
		if err != nil {
			log.Error("Failed to create http requests total counter", "error", err)
		}
		httpRequestDuration, err = meter.Float64Histogram(
			"compozy_http_request_duration_seconds",
			metric.WithDescription("HTTP request latency"),
			metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
		)
		if err != nil {
			log.Error("Failed to create http request duration histogram", "error", err)
		}
		httpRequestsInFlight, err = meter.Int64UpDownCounter(
			"compozy_http_requests_in_flight",
			metric.WithDescription("Currently active HTTP requests"),
		)
		if err != nil {
			log.Error("Failed to create http requests in flight counter", "error", err)
		}
	})
}

// ResetMetricsForTesting resets the metrics initialization state for testing.
// This function should only be used in tests to ensure clean state between test runs.
// While it's exported and available in all builds, it should not be called in production code.
func ResetMetricsForTesting() {
	initMutex.Lock()
	defer initMutex.Unlock()
	httpRequestsTotal = nil
	httpRequestDuration = nil
	httpRequestsInFlight = nil
	initOnce = sync.Once{}
}

// HTTPMetrics returns a Gin middleware that collects HTTP metrics
func HTTPMetrics(ctx context.Context, meter metric.Meter) gin.HandlerFunc {
	// Initialize metrics on first use
	initMetrics(ctx, meter)
	return func(c *gin.Context) {
		// Add logger to request context
		reqCtx := logger.ContextWithLogger(c.Request.Context(), logger.FromContext(ctx))
		c.Request = c.Request.WithContext(reqCtx)
		// Skip metrics collection if instruments are not initialized
		if httpRequestsTotal == nil {
			c.Next()
			return
		}
		// Wrap the entire middleware in a recovery to prevent panics from affecting requests
		defer func() {
			if r := recover(); r != nil {
				log := logger.FromContext(c.Request.Context())
				log.Error("Panic in HTTP metrics middleware", "panic", r)
			}
		}()
		start := time.Now()
		if httpRequestsInFlight != nil {
			httpRequestsInFlight.Add(c.Request.Context(), 1)
			defer httpRequestsInFlight.Add(c.Request.Context(), -1)
		}
		c.Next()
		recordMetrics(c, start)
	}
}

// recordMetrics records HTTP metrics after request completion
func recordMetrics(c *gin.Context, start time.Time) {
	duration := time.Since(start).Seconds()
	path := c.FullPath()
	if path == "" {
		path = "unmatched"
	}

	attrs := metric.WithAttributes(
		semconv.HTTPMethodKey.String(c.Request.Method),
		semconv.HTTPRouteKey.String(path),
		semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
	)

	if httpRequestsTotal != nil {
		httpRequestsTotal.Add(c.Request.Context(), 1, attrs)
	}
	if httpRequestDuration != nil {
		httpRequestDuration.Record(c.Request.Context(), duration, attrs)
	}
}
