---
status: pending
---

# Task 2.0: Implement HTTP Metrics Collection

## Overview

Implement HTTP metrics collection using OpenTelemetry Gin middleware to track request rates, latencies, and active requests with proper route templating to prevent high cardinality.

## Subtasks

- [ ] 2.1 Create `middleware/gin.go` for HTTP middleware implementation
- [ ] 2.2 Define HTTP metrics (requests_total, request_duration_seconds, requests_in_flight) with proper OTEL registration
- [ ] 2.3 Implement `GinMiddleware()` method using otelgin with route templating (.WithRouteTag)
- [ ] 2.4 Add proper label handling for method, path (templated), and status_code
- [ ] 2.5 Implement request timing logic and in-flight request tracking
- [ ] 2.6 Add error handling to prevent middleware failures from affecting requests
- [ ] 2.7 Create comprehensive unit tests for middleware functionality including edge cases
- [ ] 2.7a Add a specific test case to verify that requests to non-templated routes (e.g., a default 404 handler) do not generate high-cardinality path labels

## Implementation Details

### Metric Definitions

Based on the tech spec (lines 168-193 and metric reference lines 503-505), implement these metrics:

```go
// In middleware/gin.go
var (
    httpRequestsTotal    metric.Int64Counter
    httpRequestDuration  metric.Float64Histogram
    httpRequestsInFlight metric.Int64UpDownCounter
    initOnce            sync.Once
)

func initMetrics(meter metric.Meter) {
    initOnce.Do(func() {
        httpRequestsTotal, _ = meter.Int64Counter(
            "compozy_http_requests_total",
            metric.WithDescription("Total HTTP requests"),
        )
        httpRequestDuration, _ = meter.Float64Histogram(
            "compozy_http_request_duration_seconds",
            metric.WithDescription("HTTP request latency"),
            metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
        )
        httpRequestsInFlight, _ = meter.Int64UpDownCounter(
            "compozy_http_requests_in_flight",
            metric.WithDescription("Currently active HTTP requests"),
        )
    })
}
```

### Label Requirements

From the allow-list (lines 55-59):

- HTTP metrics can only use: `method`, `path`, `status_code`
- The `path` label MUST use parameterized route templates

### Middleware Implementation

Key requirements from the tech spec:

1. **Route Templating** (line 68): HTTP middleware must use `.WithRouteTag` to ensure parameterized routes (e.g., `/api/v1/users/:id`) rather than high-cardinality actual paths

2. **Integration with GinMiddleware()** (lines 113-115):

```go
// GinMiddleware returns a Gin middleware for HTTP metrics with proper route templating.
func (m *MonitoringService) GinMiddleware() gin.HandlerFunc {
    // Uses otelgin with .WithRouteTag to ensure parameterized paths
}
```

3. **Error Handling** (lines 264-267):

- Middleware errors must not abort requests
- Catch all instrumentation errors and log via `pkg/logger.Error`
- Never propagate monitoring errors to business logic

### Histogram Buckets

Use the default buckets specified in line 67:

```
[.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10]
```

The 10s upper bound covers typical API operations.

### Middleware Flow

1. Increment in-flight requests counter on entry
2. Record start time
3. Process request (call `c.Next()`)
4. Calculate duration
5. Record metrics with appropriate labels
6. Decrement in-flight requests counter

### Critical Implementation Points

1. **High Cardinality Prevention**:

    - Use Gin's `c.FullPath()` to get the route template, not `c.Request.URL.Path`
    - This ensures `/users/123` is recorded as `/users/:id`

2. **Error Recovery**:

    - Wrap all metric operations in defer/recover blocks
    - Log any panics but continue request processing

3. **Label Extraction**:
    - Method: `c.Request.Method`
    - Path: `c.FullPath()` (templated)
    - Status Code: `strconv.Itoa(c.Writer.Status())`

### Testing Requirements

1. **Unit Tests** (lines 319-334):

    - Use `t.Run("Should...")` pattern
    - Test successful request metric recording
    - Test error scenarios (panic recovery)
    - Test in-flight counter accuracy

2. **High Cardinality Test** (task 2.7a):

    - Verify that 404 requests don't create unique path labels
    - Test that dynamic routes use templates

3. **Negative Tests**:
    - Middleware panic doesn't abort request
    - Missing route template handling
    - Concurrent request handling

### Performance Considerations

From line 66: Total overhead must be <0.5% under normal load. The middleware should:

- Minimize allocations
- Use efficient label handling
- Avoid blocking operations

## Success Criteria

- HTTP metrics properly defined and initialized
- Middleware correctly records all three metrics
- Route templating prevents high cardinality
- Error handling prevents request disruption
- All tests passing including cardinality prevention
- Performance overhead within acceptable limits
