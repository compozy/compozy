# `monitoring` – _OpenTelemetry-based observability and Prometheus metrics_

> **Production-ready monitoring infrastructure providing OpenTelemetry integration, Prometheus metrics, and comprehensive observability for workflow orchestration.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Basic Monitoring Setup](#basic-monitoring-setup)
  - [HTTP Middleware](#http-middleware)
  - [Temporal Interceptors](#temporal-interceptors)
  - [Custom Metrics](#custom-metrics)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `monitoring` package provides comprehensive observability infrastructure for the Compozy workflow orchestration engine. It integrates OpenTelemetry for distributed tracing and metrics, Prometheus for metric collection, and provides middleware for HTTP servers and Temporal workflows.

Key capabilities include:

- **OpenTelemetry Integration**: Standardized observability with distributed tracing
- **Prometheus Metrics**: Production-ready metrics collection and exposition
- **HTTP Monitoring**: Gin middleware for request metrics and tracing
- **Temporal Monitoring**: Workflow and activity metrics via interceptors
- **System Health**: Memory, dispatcher, and system-level metrics

---

## 💡 Motivation

- **Production Observability**: Essential monitoring for production workflow orchestration
- **Standardized Metrics**: OpenTelemetry-based metrics for industry standard observability
- **Performance Insights**: Detailed HTTP request, workflow, and system performance tracking
- **Graceful Degradation**: Fallback to no-op implementations when monitoring fails

---

## ⚡ Design Highlights

- **Service-Based Architecture**: Centralized monitoring service with clear lifecycle management
- **Graceful Fallback**: Continues operation even when monitoring initialization fails
- **Middleware Integration**: Seamless integration with Gin HTTP and Temporal workflow frameworks
- **Memory Monitoring**: Advanced memory usage tracking and leak detection
- **Configurable Endpoints**: Flexible metrics endpoint configuration

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/compozy/compozy/engine/infra/monitoring"
    "github.com/gin-gonic/gin"
)

func main() {
    ctx := context.Background()

    // Create monitoring service
    config := &monitoring.Config{
        Enabled: true,
        Path:    "/metrics",
    }

    service, err := monitoring.NewMonitoringService(ctx, config)
    if err != nil {
        log.Fatal("Failed to initialize monitoring:", err)
    }
    defer service.Shutdown(ctx)

    // Setup HTTP server with monitoring
    router := gin.New()
    router.Use(service.GinMiddleware(ctx))

    // Add metrics endpoint
    router.GET(config.Path, gin.WrapH(service.ExporterHandler()))

    // Start server
    log.Println("Server starting on :5001")
    http.ListenAndServe(":5001", router)
}
```

---

## 📖 Usage

### Basic Monitoring Setup

```go
// Initialize monitoring service
config := &monitoring.Config{
    Enabled: true,
    Path:    "/metrics",
}

service, err := monitoring.NewMonitoringService(ctx, config)
if err != nil {
    return fmt.Errorf("failed to initialize monitoring: %w", err)
}

// Set as global OpenTelemetry provider
service.SetAsGlobal()

// Get meter for custom metrics
meter := service.Meter()
```

### HTTP Middleware

```go
// Setup Gin router with monitoring middleware
router := gin.New()
router.Use(service.GinMiddleware(ctx))

// Add metrics endpoint
router.GET("/metrics", gin.WrapH(service.ExporterHandler()))

// Your routes will now be automatically monitored
router.GET("/api/v1/workflows", getWorkflows)
router.POST("/api/v1/workflows", createWorkflow)
```

### Temporal Interceptors

```go
// Setup Temporal worker with monitoring
worker := worker.New(client, "task-queue", worker.Options{
    Interceptors: []interceptor.WorkerInterceptor{
        service.TemporalInterceptor(ctx),
    },
})

// Workflows and activities will be automatically monitored
worker.RegisterWorkflow(MyWorkflow)
worker.RegisterActivity(MyActivity)
```

### Custom Metrics

```go
// Create custom metrics using the service meter
meter := service.Meter()

// Counter metric
counter, err := meter.Int64Counter(
    "workflow_executions_total",
    metric.WithDescription("Total number of workflow executions"),
)
if err != nil {
    return err
}

// Increment counter
counter.Add(ctx, 1, metric.WithAttributes(
    attribute.String("workflow_type", "data_processing"),
    attribute.String("status", "success"),
))

// Histogram metric
histogram, err := meter.Float64Histogram(
    "workflow_duration_seconds",
    metric.WithDescription("Workflow execution duration"),
)
if err != nil {
    return err
}

// Record measurement
histogram.Record(ctx, duration.Seconds(), metric.WithAttributes(
    attribute.String("workflow_type", "data_processing"),
))
```

---

## 🔧 Configuration

### Environment Variables

```bash
# Enable/disable monitoring
MONITORING_ENABLED=true

# Metrics endpoint path
MONITORING_PATH=/metrics
```

### Programmatic Configuration

```go
config := &monitoring.Config{
    Enabled: true,
    Path:    "/metrics",
}

// Load configuration with environment variable override
config, err := monitoring.LoadWithEnv(ctx, config)
if err != nil {
    return fmt.Errorf("failed to load monitoring config: %w", err)
}
```

### Default Configuration

```go
// Get default configuration
config := monitoring.DefaultConfig()
// Returns: &Config{Enabled: false, Path: "/metrics"}
```

---

## 🎨 Examples

### Production Server Setup

```go
func setupMonitoringServer(ctx context.Context) error {
    // Initialize monitoring with fallback
    config := &monitoring.Config{
        Enabled: true,
        Path:    "/metrics",
    }

    service := monitoring.NewMonitoringServiceWithFallback(ctx, config)

    // Setup HTTP server
    router := gin.New()
    router.Use(gin.Recovery())
    router.Use(service.GinMiddleware(ctx))

    // Health check endpoint
    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "status": "ok",
            "monitoring": service.IsInitialized(),
        })
    })

    // Metrics endpoint
    router.GET(config.Path, gin.WrapH(service.ExporterHandler()))

    return http.ListenAndServe(":5001", router)
}
```

### Custom Metrics Dashboard

```go
func createCustomMetrics(service *monitoring.Service) error {
    meter := service.Meter()

    // Business metrics
    workflowCounter, err := meter.Int64Counter(
        "workflows_processed_total",
        metric.WithDescription("Total workflows processed"),
    )
    if err != nil {
        return err
    }

    taskDuration, err := meter.Float64Histogram(
        "task_execution_duration_seconds",
        metric.WithDescription("Task execution duration"),
    )
    if err != nil {
        return err
    }

    errorRate, err := meter.Float64Counter(
        "workflow_errors_total",
        metric.WithDescription("Total workflow errors"),
    )
    if err != nil {
        return err
    }

    // Use metrics in your application
    workflowCounter.Add(ctx, 1, metric.WithAttributes(
        attribute.String("type", "data_processing"),
        attribute.String("status", "completed"),
    ))

    return nil
}
```

### Memory Monitoring Integration

```go
func setupMemoryMonitoring(
    service *monitoring.Service,
    memoryManager *memory.Manager,
) *monitoring.MemoryMonitoringInterceptor {
    // Create memory monitoring interceptor
    interceptor := monitoring.NewMemoryMonitoringInterceptor(memoryManager)

    // Initialize memory monitoring metrics
    monitoring.InitializeMemoryMonitoring(ctx, service.Meter())

    return interceptor
}
```

### Dispatcher Health Monitoring

```go
func monitorDispatcherHealth(service *monitoring.Service) {
    // Initialize dispatcher health metrics
    monitoring.InitDispatcherHealthMetrics(ctx, service.Meter())

    // Register dispatcher
    dispatcherID := "dispatcher-1"
    staleThreshold := 30 * time.Second
    monitoring.RegisterDispatcher(ctx, dispatcherID, staleThreshold)

    // Update heartbeat periodically
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            monitoring.UpdateDispatcherHeartbeat(ctx, dispatcherID)
        case <-ctx.Done():
            monitoring.UnregisterDispatcher(ctx, dispatcherID)
            return
        }
    }
}
```

### System Metrics Monitoring

```go
func setupSystemMetrics(service *monitoring.Service) {
    // Initialize system metrics
    monitoring.InitSystemMetrics(ctx, service.Meter())

    // System metrics are automatically collected
    // Access metrics programmatically if needed
    healthyDispatchers := monitoring.GetHealthyDispatcherCount()
    staleDispatchers := monitoring.GetStaleDispatcherCount()

    log.Printf("Healthy dispatchers: %d, Stale dispatchers: %d",
        healthyDispatchers, staleDispatchers)
}
```

---

## 📚 API Reference

### Core Types

#### `Service`

Main monitoring service providing observability infrastructure.

```go
type Service struct {
    // private fields
}

func NewMonitoringService(ctx context.Context, cfg *Config) (*Service, error)
func NewMonitoringServiceWithFallback(ctx context.Context, cfg *Config) *Service
func (s *Service) Meter() metric.Meter
func (s *Service) GinMiddleware(ctx context.Context) gin.HandlerFunc
func (s *Service) TemporalInterceptor(ctx context.Context) interceptor.WorkerInterceptor
func (s *Service) ExporterHandler() http.Handler
func (s *Service) Shutdown(ctx context.Context) error
func (s *Service) IsInitialized() bool
func (s *Service) InitializationError() error
func (s *Service) SetAsGlobal()
```

#### `Config`

Configuration for monitoring service.

```go
type Config struct {
    Enabled bool   `json:"enabled" yaml:"enabled" env:"MONITORING_ENABLED"`
    Path    string `json:"path"    yaml:"path"    env:"MONITORING_PATH"`
}

func DefaultConfig() *Config
func LoadWithEnv(ctx context.Context, yamlConfig *Config) (*Config, error)
func (c *Config) Validate() error
```

### System Metrics

#### System Health Functions

```go
func InitSystemMetrics(ctx context.Context, meter metric.Meter)
func ResetSystemMetricsForTesting(ctx context.Context)
```

#### Dispatcher Health Functions

```go
type DispatcherHealth struct {
    ID            string
    LastHeartbeat time.Time
    IsStale       bool
    StaleThreshold time.Duration
}

func InitDispatcherHealthMetrics(ctx context.Context, meter metric.Meter)
func RegisterDispatcher(ctx context.Context, dispatcherID string, staleThreshold time.Duration)
func UnregisterDispatcher(ctx context.Context, dispatcherID string)
func UpdateDispatcherHeartbeat(ctx context.Context, dispatcherID string)
func GetDispatcherHealth(dispatcherID string) (*DispatcherHealth, bool)
func GetAllDispatcherHealth() map[string]*DispatcherHealth
func GetHealthyDispatcherCount() int
func GetStaleDispatcherCount() int
func ResetDispatcherHealthMetricsForTesting(ctx context.Context)
```

### Memory Monitoring

#### Memory Monitoring Functions

```go
func InitializeMemoryMonitoring(ctx context.Context, meter metric.Meter)
func NewMemoryMonitoringInterceptor(manager *memory.Manager) *MemoryMonitoringInterceptor

type MemoryMonitoringInterceptor struct {
    // private fields
}
```

### Middleware and Interceptors

#### HTTP Metrics Handler

```go
func MetricsHandler() // Returns HTTP handler for metrics endpoint
```

---

## 🧪 Testing

### Unit Tests

```go
func TestMonitoringService(t *testing.T) {
    ctx := context.Background()

    t.Run("Should initialize with valid config", func(t *testing.T) {
        config := &monitoring.Config{
            Enabled: true,
            Path:    "/metrics",
        }

        service, err := monitoring.NewMonitoringService(ctx, config)
        require.NoError(t, err)
        assert.True(t, service.IsInitialized())

        defer service.Shutdown(ctx)
    })

    t.Run("Should fallback gracefully on initialization failure", func(t *testing.T) {
        config := &monitoring.Config{
            Enabled: true,
            Path:    "invalid-path", // Invalid path
        }

        service := monitoring.NewMonitoringServiceWithFallback(ctx, config)
        assert.False(t, service.IsInitialized())
        assert.NotNil(t, service.InitializationError())
    })
}
```

### Integration Tests

```go
func TestMonitoringIntegration(t *testing.T) {
    ctx := context.Background()

    // Setup monitoring service
    service, err := monitoring.NewMonitoringService(ctx, &monitoring.Config{
        Enabled: true,
        Path:    "/metrics",
    })
    require.NoError(t, err)
    defer service.Shutdown(ctx)

    // Setup HTTP server with monitoring
    router := gin.New()
    router.Use(service.GinMiddleware(ctx))
    router.GET("/metrics", gin.WrapH(service.ExporterHandler()))
    router.GET("/test", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })

    // Test metrics collection
    server := httptest.NewServer(router)
    defer server.Close()

    // Make request to generate metrics
    resp, err := http.Get(server.URL + "/test")
    require.NoError(t, err)
    resp.Body.Close()

    // Check metrics endpoint
    metricsResp, err := http.Get(server.URL + "/metrics")
    require.NoError(t, err)
    defer metricsResp.Body.Close()

    assert.Equal(t, 200, metricsResp.StatusCode)

    // Verify metrics content
    body, err := io.ReadAll(metricsResp.Body)
    require.NoError(t, err)
    assert.Contains(t, string(body), "http_requests_total")
}
```

### Test Utilities

```go
func setupTestMonitoring(t *testing.T) *monitoring.Service {
    ctx := context.Background()
    service, err := monitoring.NewMonitoringService(ctx, &monitoring.Config{
        Enabled: true,
        Path:    "/metrics",
    })
    require.NoError(t, err)

    t.Cleanup(func() {
        service.Shutdown(ctx)
    })

    return service
}

func resetMetricsForTesting(t *testing.T) {
    ctx := context.Background()
    monitoring.ResetSystemMetricsForTesting(ctx)
    monitoring.ResetDispatcherHealthMetricsForTesting(ctx)
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../../LICENSE)
