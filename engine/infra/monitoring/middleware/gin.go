package middleware

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
)

var (
	httpRequestsTotal    metric.Int64Counter
	httpRequestDuration  metric.Float64Histogram
	httpRequestSize      metric.Int64Histogram
	httpResponseSize     metric.Int64Histogram
	httpRequestsInFlight metric.Int64UpDownCounter
	initOnce             sync.Once
	initMutex            sync.Mutex
)

// initMetrics initializes the HTTP metrics instruments
func initMetrics(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	initOnce.Do(func() {
		initMutex.Lock()
		defer initMutex.Unlock()
		initializeHTTPMetrics(ctx, meter)
	})
}

// initializeHTTPMetrics configures HTTP metric instruments and logs failures.
func initializeHTTPMetrics(ctx context.Context, meter metric.Meter) {
	log := logger.FromContext(ctx)
	var err error
	httpRequestsTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("http", "requests"),
		metric.WithDescription("Total HTTP requests"),
	)
	if err != nil {
		log.Error("Failed to create http requests total counter", "error", err)
	}
	httpRequestDuration, err = meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("http", "request_duration_seconds"),
		metric.WithDescription("HTTP request latency"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
	)
	if err != nil {
		log.Error("Failed to create http request duration histogram", "error", err)
	}
	httpRequestSize, err = meter.Int64Histogram(
		metrics.MetricNameWithSubsystem("http", "request_size_bytes"),
		metric.WithDescription("Size distribution of HTTP request bodies"),
		metric.WithUnit("bytes"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000, 10000000, 100000000),
	)
	if err != nil {
		log.Error("Failed to create http request size histogram", "error", err)
	}
	httpResponseSize, err = meter.Int64Histogram(
		metrics.MetricNameWithSubsystem("http", "response_size_bytes"),
		metric.WithDescription("Size distribution of HTTP response bodies"),
		metric.WithUnit("bytes"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000, 10000000, 100000000),
	)
	if err != nil {
		log.Error("Failed to create http response size histogram", "error", err)
	}
	httpRequestsInFlight, err = meter.Int64UpDownCounter(
		metrics.MetricNameWithSubsystem("http", "requests_in_flight"),
		metric.WithDescription("Currently active HTTP requests"),
	)
	if err != nil {
		log.Error("Failed to create http requests in flight counter", "error", err)
	}
}

// ResetMetricsForTesting resets the metrics initialization state for testing.
// This function should only be used in tests to ensure clean state between test runs.
// While it's exported and available in all builds, it should not be called in production code.
func ResetMetricsForTesting() {
	initMutex.Lock()
	defer initMutex.Unlock()
	httpRequestsTotal = nil
	httpRequestDuration = nil
	httpRequestSize = nil
	httpResponseSize = nil
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
		countedBody := wrapRequestBody(c.Request)
		if httpRequestsInFlight != nil {
			attrs := metric.WithAttributes(
				semconv.HTTPMethodKey.String(c.Request.Method),
			)
			httpRequestsInFlight.Add(c.Request.Context(), 1, attrs)
			defer httpRequestsInFlight.Add(c.Request.Context(), -1, attrs)
		}
		c.Next()
		recordMetrics(c, start, countedBody)
	}
}

// recordMetrics records HTTP metrics after request completion
func recordMetrics(c *gin.Context, start time.Time, countedBody *bodyReadCounter) {
	duration := time.Since(start).Seconds()
	path := c.FullPath()
	if path == "" {
		path = "unmatched"
	}

	labels := []attribute.KeyValue{
		semconv.HTTPMethodKey.String(c.Request.Method),
		semconv.HTTPRouteKey.String(path),
		semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
	}
	attrs := metric.WithAttributes(labels...)

	if httpRequestsTotal != nil {
		httpRequestsTotal.Add(c.Request.Context(), 1, attrs)
	}
	if httpRequestDuration != nil {
		httpRequestDuration.Record(c.Request.Context(), duration, attrs)
	}
	if httpRequestSize != nil {
		size := requestSizeBytes(c, countedBody)
		httpRequestSize.Record(c.Request.Context(), size, attrs)
	}
	if httpResponseSize != nil {
		size := responseSizeBytes(c.Writer)
		httpResponseSize.Record(c.Request.Context(), size, attrs)
	}
}

func requestSizeBytes(c *gin.Context, countedBody *bodyReadCounter) int64 {
	if countedBody != nil {
		if read := countedBody.BytesRead(); read > 0 {
			return read
		}
	}
	if c.Request.ContentLength > 0 {
		return c.Request.ContentLength
	}
	if header := c.Request.Header.Get("Content-Length"); header != "" {
		if parsed, err := strconv.ParseInt(header, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

func responseSizeBytes(writer gin.ResponseWriter) int64 {
	size := int64(writer.Size())
	if size >= 0 {
		return size
	}
	if header := writer.Header().Get("Content-Length"); header != "" {
		if parsed, err := strconv.ParseInt(header, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

// bodyReadCounter wraps an io.ReadCloser and tracks the bytes read from it.
type bodyReadCounter struct {
	io.ReadCloser
	bytesRead int64
}

func (r *bodyReadCounter) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.bytesRead += int64(n)
	}
	return n, err
}

func (r *bodyReadCounter) BytesRead() int64 {
	return r.bytesRead
}

func wrapRequestBody(req *http.Request) *bodyReadCounter {
	if req == nil || req.Body == nil {
		return nil
	}
	if existing, ok := req.Body.(*bodyReadCounter); ok {
		return existing
	}
	counter := &bodyReadCounter{ReadCloser: req.Body}
	req.Body = counter
	return counter
}
