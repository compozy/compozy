---
status: completed
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 1.0: Set Up Core Monitoring Infrastructure

## Overview

Create the foundational monitoring package structure and implement the core MonitoringService that will provide centralized observability for Compozy.

## Subtasks

- [x] 1.1 Create the `engine/infra/monitoring` package directory structure
- [x] 1.2 Add OpenTelemetry dependencies to `go.mod` (otel SDK, Prometheus exporter, otelgin)
- [x] 1.3 Create `config.go` with `Config` struct and `DefaultConfig()` function
- [x] 1.4 Implement `monitoring.go` with `MonitoringService` struct and constructor following mandatory patterns
- [x] 1.5 Initialize MeterProvider with Prometheus exporter in the constructor
- [x] 1.6 Implement graceful fallback to no-op meter on initialization failures
- [x] 1.6a Update the `ExporterHandler` to return an HTTP 503 Service Unavailable status if the monitoring service failed to initialize
- [x] 1.7 Create unit tests for MonitoringService initialization with positive and negative cases

## Implementation Details

### Package Structure

As specified in the tech spec (lines 11-16), create:

```
engine/infra/monitoring/
├── monitoring.go         # Core service implementation
├── monitoring_test.go    # Service tests
├── config.go            # Configuration types
├── config_test.go       # Configuration tests
├── middleware/          # HTTP middleware
└── interceptor/         # Temporal interceptors
```

### Dependencies (go.mod)

Add these dependencies as specified in line 434-438:

```go
go.opentelemetry.io/otel
go.opentelemetry.io/otel/exporters/prometheus
go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin
```

### Configuration Structure

Implement the configuration as defined in lines 452-463:

```go
type Config struct {
    Enabled bool   `yaml:"enabled" env:"MONITORING_ENABLED" default:"false"`
    Path    string `yaml:"path" default:"/metrics"`
}

func DefaultConfig() *Config {
    return &Config{
        Enabled: false,
        Path:    "/metrics",
    }
}
```

### Core Service Implementation

Follow the interface design from lines 89-131:

```go
// MonitoringService encapsulates instrumentation logic.
type MonitoringService struct {
    meter    metric.Meter
    exporter *prometheus.Exporter
    provider *metric.MeterProvider
    // configuration fields
}

// Constructor must follow mandatory patterns (lines 98-105, 136-158)
func NewMonitoringService(ctx context.Context, cfg *Config) (*MonitoringService, error) {
    if cfg == nil {
        cfg = DefaultConfig()
    }

    // Initialize Prometheus exporter
    exporter, err := prometheus.New()
    if err != nil {
        log.Error("Failed to initialize Prometheus exporter", "error", err)
        // Return service with no-op meter for graceful degradation
        return &MonitoringService{meter: noop.NewMeterProvider().Meter("noop")}, nil
    }

    // Create MeterProvider with exporter
    provider := metric.NewMeterProvider(metric.WithReader(exporter))
    meter := provider.Meter("compozy")

    return &MonitoringService{
        meter:    meter,
        exporter: exporter,
        provider: provider,
    }, nil
}
```

### Key Methods to Implement

- `Meter()`: Returns the OpenTelemetry meter for custom instrumentation
- `GinMiddleware()`: Returns Gin middleware (stub for now)
- `TemporalInterceptor()`: Returns Temporal interceptor (stub for now)
- `ExporterHandler()`: Returns http.Handler for /metrics endpoint
- `Shutdown()`: Graceful cleanup (no-op as we reuse main server)

### Error Handling Requirements

As specified in lines 256-273:

- Log all errors via `pkg/logger.Error`
- Return no-op implementations on failure
- Never propagate monitoring errors to business logic
- ExporterHandler must return 503 if initialization failed

### Testing Requirements

Follow project standards (lines 316-334):

- Use `t.Run("Should...")` pattern
- Use `testify/assert` for assertions
- Use `oteltest.NewMeterProvider()` for hermetic testing
- Include both positive and negative test cases

### Context Handling

Per line 49: All service methods performing I/O or long-running operations must accept `context.Context` as first parameter.

## Success Criteria

- Package structure created correctly
- Dependencies added and building successfully
- Configuration supports both environment variables and YAML
- Service initializes with Prometheus exporter
- Graceful fallback to no-op on initialization failure
- ExporterHandler returns 503 on failure
- All tests passing with proper coverage
- Follows all project coding standards

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test-all` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
