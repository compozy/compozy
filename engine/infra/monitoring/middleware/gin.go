package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	httpRequestsTotal    metric.Int64Counter
	httpRequestDuration  metric.Float64Histogram
	httpRequestsInFlight metric.Int64UpDownCounter
	initOnce             sync.Once
	initMutex            sync.Mutex
)

// initMetrics initializes the HTTP metrics instruments
func initMetrics(meter metric.Meter) {
	// Skip initialization if meter is nil
	if meter == nil {
		return
	}

	initOnce.Do(func() {
		var err error
		httpRequestsTotal, err = meter.Int64Counter(
			"compozy_http_requests_total",
			metric.WithDescription("Total HTTP requests"),
		)
		if err != nil {
			logger.Error("Failed to create http requests total counter", "error", err)
		}
		httpRequestDuration, err = meter.Float64Histogram(
			"compozy_http_request_duration_seconds",
			metric.WithDescription("HTTP request latency"),
			metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
		)
		if err != nil {
			logger.Error("Failed to create http request duration histogram", "error", err)
		}
		httpRequestsInFlight, err = meter.Int64UpDownCounter(
			"compozy_http_requests_in_flight",
			metric.WithDescription("Currently active HTTP requests"),
		)
		if err != nil {
			logger.Error("Failed to create http requests in flight counter", "error", err)
		}
	})
}

// ResetMetricsForTesting resets the metrics initialization state for testing
// This should only be used in tests to ensure clean state between test runs
func ResetMetricsForTesting() {
	initMutex.Lock()
	defer initMutex.Unlock()
	httpRequestsTotal = nil
	httpRequestDuration = nil
	httpRequestsInFlight = nil
	initOnce = sync.Once{}
}

// HTTPMetrics returns a Gin middleware that collects HTTP metrics
func HTTPMetrics(meter metric.Meter) gin.HandlerFunc {
	// Initialize metrics on first use
	initMetrics(meter)

	return func(c *gin.Context) {
		// Skip metrics collection if instruments are not initialized
		if httpRequestsTotal == nil {
			c.Next()
			return
		}

		// Wrap the entire middleware in a recovery to prevent panics from affecting requests
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic in HTTP metrics middleware", "panic", r)
			}
		}()

		start := time.Now()
		httpRequestsInFlight.Add(c.Request.Context(), 1)
		defer httpRequestsInFlight.Add(c.Request.Context(), -1)

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
		attribute.String("method", c.Request.Method),
		attribute.String("path", path),
		attribute.String("status_code", strconv.Itoa(c.Writer.Status())),
	)

	httpRequestsTotal.Add(c.Request.Context(), 1, attrs)
	httpRequestDuration.Record(c.Request.Context(), duration, attrs)
}
