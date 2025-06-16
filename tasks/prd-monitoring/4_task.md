---
status: completed
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 4.0: Add System Health Metrics

## Overview

Implement system health metrics to expose build information and service uptime, providing essential metadata for operational monitoring.

## Subtasks

- [x] 4.1 Define build-time ldflags strategy for version and commit injection
- [x] 4.2 Add build_info gauge with version, commit_hash, and go_version labels
- [x] 4.3 Implement uptime_seconds_total counter with monotonic behavior
- [x] 4.4 Create build info extraction from ldflags or fallback to runtime
- [x] 4.5 Initialize system metrics on service startup
- [x] 4.6 Update Makefile to inject build variables during compilation
- [x] 4.7 Create tests to verify metric values and label correctness

## Implementation Details

### Metric Definitions

Based on the tech spec (lines 512-513), implement these metrics:

```go
// System health metrics
var (
    buildInfo    metric.Float64Gauge
    uptimeTotal  metric.Int64Counter
    startTime    time.Time
)

func initSystemMetrics(meter metric.Meter) {
    buildInfo, _ = meter.Float64Gauge(
        "compozy_build_info",
        metric.WithDescription("Build information (value=1)"),
    )

    uptimeGauge, _ = meter.Float64ObservableGauge(
        "compozy_uptime_seconds",
        metric.WithDescription("Service uptime in seconds"),
    )

    // Record start time for uptime calculation
    startTime = time.Now()
}
```

### Label Requirements

From the allow-list (lines 55-59):

- System metrics can only use: `version`, `commit_hash`, `go_version`

### Build Info Strategy

1. **Build Variables**:

```go
// These will be set via ldflags
var (
    Version    = "unknown"
    CommitHash = "unknown"
    BuildTime  = "unknown"
)
```

2. **Makefile Updates** (task 4.6):

```makefile
# Get build info
GIT_COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(shell git describe --tags --always)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS := -X 'main.Version=$(VERSION)' \
           -X 'main.CommitHash=$(GIT_COMMIT)' \
           -X 'main.BuildTime=$(BUILD_TIME)'

build:
    go build -ldflags "$(LDFLAGS)" -o compozy ./cmd/compozy
```

3. **Runtime Fallback**:

```go
func getBuildInfo() (version, commit, goVersion string) {
    // Primary: Use injected build variables
    version = Version
    commit = CommitHash

    // Fallback: Try to get from runtime
    if version == "unknown" {
        if info, ok := debug.ReadBuildInfo(); ok {
            version = info.Main.Version
        }
    }

    // Go version from runtime
    goVersion = runtime.Version()

    return version, commit, goVersion
}
```

### Build Info Metric Implementation

The `build_info` metric follows the Prometheus pattern of using a gauge with value=1 and encoding information in labels:

```go
func recordBuildInfo(ctx context.Context) {
    version, commit, goVersion := getBuildInfo()

    buildInfo.Record(ctx, 1,
        metric.WithAttributes(
            attribute.String("version", version),
            attribute.String("commit_hash", commit),
            attribute.String("go_version", goVersion),
        ),
    )
}
```

### Uptime Counter Implementation

The uptime counter must be monotonic and reset on restart:

```go
func initUptimeCounter(ctx context.Context) {
    // Start a goroutine to update uptime every second
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                uptime := int64(time.Since(startTime).Seconds())
                uptimeTotal.Add(ctx, 1)
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

Alternative approach using a callback gauge:

```go
func registerUptimeGauge(meter metric.Meter) {
    uptimeGauge, _ := meter.Float64ObservableGauge(
        "compozy_uptime_seconds_total",
        metric.WithDescription("Service uptime in seconds"),
    )

    _, err := meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
        uptime := time.Since(startTime).Seconds()
        o.ObserveFloat64(uptimeGauge, uptime)
        return nil
    }, uptimeGauge)
}
```

### Integration with MonitoringService

Add system metrics initialization to the service:

```go
func (m *MonitoringService) InitSystemMetrics(ctx context.Context) {
    initSystemMetrics(m.meter)
    recordBuildInfo(ctx)
    initUptimeCounter(ctx)
}
```

### Testing Requirements

1. **Build Info Tests**:

    - Test with injected values
    - Test fallback behavior
    - Verify label correctness

2. **Uptime Tests**:

    - Verify monotonic increase
    - Test counter reset on restart
    - Check accuracy of time calculation

3. **Label Validation**:
    - Ensure only allowed labels are used
    - Test special characters in version strings

### Bootstrap Logging

From lines 368-370:

```go
logger.Info("System metrics initialized",
    "version", version,
    "commit", commit,
    "go_version", goVersion,
)
```

## Success Criteria

- Build info correctly extracted from ldflags
- Fallback mechanism works when ldflags not set
- Makefile properly injects build variables
- Uptime counter increases monotonically
- Both metrics use only allowed labels
- Tests verify all scenarios
- Metrics visible at /metrics endpoint

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
