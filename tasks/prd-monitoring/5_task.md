---
status: pending
---

# Task 5.0: Integrate Monitoring Service with Main Application

## Overview

Integrate the MonitoringService into the main application server and Temporal workers, ensuring proper dependency injection and endpoint registration.

## Subtasks

- [ ] 5.1 Update `infra/server` package to import and initialize MonitoringService
- [ ] 5.2 Add monitoring service dependency injection to server constructor
- [ ] 5.3 Register `/metrics` endpoint on main Gin router with ExporterHandler()
- [ ] 5.4 Add monitoring middleware to Gin router before other middleware
- [ ] 5.5 Update Temporal worker initialization to include monitoring interceptor
- [ ] 5.6 Implement proper error handling for monitoring initialization failures
- [ ] 5.7 Add Swagger documentation for `/metrics` endpoint with clear descriptions

## Implementation Details

### Server Integration

Based on the tech spec (lines 226-238), implement the integration:

```go
// In infra/server package
func NewServer(ctx context.Context, cfg *config.Config) (*Server, error) {
    // ... existing initialization ...

    // Initialize monitoring service
    ms, err := monitoring.NewMonitoringService(ctx, cfg.Monitoring)
    if err != nil {
        // Log but don't fail - monitoring is not critical
        logger.Error("Failed to initialize monitoring", "error", err)
        // Continue with nil monitoring service
    }

    // Create Gin router
    r := gin.New()

    // Add monitoring middleware BEFORE other middleware
    if ms != nil {
        r.Use(ms.GinMiddleware())
    }

    // ... other middleware ...

    // Register metrics endpoint (not versioned under /api/v0/)
    if ms != nil {
        r.GET("/metrics", gin.WrapH(ms.ExporterHandler()))
    }

    return &Server{
        router:     r,
        monitoring: ms,
        // ... other fields ...
    }, nil
}
```

### Key Integration Points

1. **Middleware Order** (line 236):

    - Add monitoring middleware BEFORE other middleware
    - This ensures all requests are tracked

2. **Endpoint Path** (line 219):

    - `/metrics` is NOT versioned under `/api/v0/`
    - It's an operational endpoint

3. **Error Handling** (lines 231-233):
    - Don't fail server startup if monitoring fails
    - Log the error and continue
    - Use nil checks before using monitoring service

### Temporal Worker Integration

Based on lines 243-252:

```go
// In engine/worker package
func NewWorker(ctx context.Context, cfg *WorkerConfig, ms *monitoring.MonitoringService) (*Worker, error) {
    // ... existing initialization ...

    workerOptions := worker.Options{
        // ... existing options ...
    }

    // Add monitoring interceptor if available
    if ms != nil {
        interceptor, err := ms.TemporalInterceptor(ctx)
        if err != nil {
            logger.Error("Failed to create Temporal interceptor", "error", err)
            // Continue without interceptor rather than failing
        } else if interceptor != nil {
            workerOptions.Interceptors = append(workerOptions.Interceptors, interceptor)
        }
    }

    // Create worker with options
    w := worker.New(temporalClient, taskQueue, workerOptions)

    return &Worker{
        worker: w,
        // ... other fields ...
    }, nil
}
```

### Dependency Injection Pattern

Update server struct to include monitoring:

```go
type Server struct {
    router     *gin.Engine
    monitoring *monitoring.MonitoringService
    db         *database.DB
    temporal   temporal.Client
    // ... other dependencies ...
}
```

Pass monitoring service to components that need it:

```go
// When creating workers
worker, err := worker.NewWorker(ctx, workerCfg, server.monitoring)

// When creating custom metric recorders in other packages
if server.monitoring != nil {
    taskCounter := metrics.TaskProcessedCounter(server.monitoring.Meter())
}
```

### Swagger Documentation

Add OpenAPI documentation for the metrics endpoint (line 222):

```yaml
/metrics:
    get:
        tags:
            - Operations
        summary: Prometheus metrics endpoint
        description: |
            Exposes application metrics in Prometheus exposition format.
            This endpoint is used by Prometheus servers to scrape metrics.

            The response is in text/plain format following the Prometheus
            exposition format specification.

            Available metrics include:
            - HTTP request rates and latencies
            - Temporal workflow execution metrics
            - System health information
        operationId: getMetrics
        responses:
            "200":
                description: Metrics in Prometheus format
                content:
                    text/plain:
                        schema:
                            type: string
                            example: |
                                # HELP compozy_http_requests_total Total HTTP requests
                                # TYPE compozy_http_requests_total counter
                                compozy_http_requests_total{method="GET",path="/api/v1/users",status_code="200"} 1234
            "503":
                description: Monitoring service unavailable
                content:
                    text/plain:
                        schema:
                            type: string
                            example: "Monitoring service not initialized"
```

### Bootstrap Logging

From lines 368-370, add startup logging:

```go
if ms != nil {
    logger.Info("Monitoring service initialized",
        "enabled", cfg.Monitoring.Enabled,
        "path", cfg.Monitoring.Path)
} else {
    logger.Warn("Monitoring service not available")
}
```

### Error Scenarios to Handle

1. **Nil Monitoring Service**:

    - Check for nil before any method calls
    - Gracefully degrade functionality

2. **Partial Initialization**:

    - Monitoring service exists but interceptor fails
    - Continue without specific features

3. **Runtime Errors**:
    - Metrics recording failures
    - Log but don't propagate

## Success Criteria

- MonitoringService properly integrated into server
- Middleware added in correct order
- `/metrics` endpoint accessible and documented
- Temporal workers include interceptor when available
- Graceful handling of monitoring failures
- Swagger documentation complete
- No critical path dependencies on monitoring
