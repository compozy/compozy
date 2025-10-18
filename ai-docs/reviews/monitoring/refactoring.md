[3 tools called]

Based on the research, here's what I found and my recommendation:

## Industry Standards:

### **Prometheus/OpenTelemetry Best Practices:**

1. ✅ **Use a namespace prefix** (like `compozy_`) to prevent metric name collisions and organize metrics by application
2. ✅ **Use snake_case** for metric names
3. ✅ **Single-word prefix** that reflects the domain/application
4. ✅ **Consistent naming** across all metrics in the application

### **Common Approaches in Go Projects:**

Both approaches are valid, but they serve different purposes:

| Approach                 | Pros                                                 | Cons                               | Use Case                                      |
| ------------------------ | ---------------------------------------------------- | ---------------------------------- | --------------------------------------------- |
| **Centralized Constant** | Single source of truth, easy to change globally      | Still need to manually concatenate | Good for application-level prefix             |
| **Helper Function**      | Enforces consistency automatically, harder to forget | Slight indirection                 | Better for ensuring prefix is never forgotten |
| **Explicit Per-Metric**  | Full control, no indirection                         | Error-prone, easy to forget prefix | Not recommended for app prefix                |

## ✅ **Recommended Approach for Compozy:**

Create a **hybrid solution** with both a constant AND a helper function:

### 1. **Create a metrics package with centralized naming:**

```go
// engine/infra/monitoring/metrics/naming.go
package metrics

import "fmt"

const (
    // MetricPrefix is the namespace prefix for all Compozy metrics
    MetricPrefix = "compozy_"
)

// MetricName builds a fully qualified metric name with the compozy_ prefix
func MetricName(name string) string {
    return MetricPrefix + name
}

// MetricNameWithSubsystem builds a metric name with subsystem: compozy_<subsystem>_<name>
func MetricNameWithSubsystem(subsystem, name string) string {
    return fmt.Sprintf("%s%s_%s", MetricPrefix, subsystem, name)
}
```

### 2. **Update all metrics to use the helper:**

**Example for auth metrics:**

```go
// engine/auth/metrics.go
import "github.com/compozy/compozy/engine/infra/monitoring/metrics"

authRequestsTotal, err = meter.Int64Counter(
    metrics.MetricName("auth_requests_total"),  // becomes "compozy_auth_requests_total"
    metric.WithDescription("Total number of auth requests"),
)
```

**Example for execution metrics (currently missing prefix):**

```go
// engine/infra/monitoring/execution_metrics.go
import "github.com/compozy/compozy/engine/infra/monitoring/metrics"

latency, err := meter.Float64Histogram(
    metrics.MetricName("http_exec_sync_latency_seconds"),  // becomes "compozy_http_exec_sync_latency_seconds"
    metric.WithDescription("Latency of synchronous execution endpoints"),
)
```

**For subsystem-specific metrics:**

```go
// engine/knowledge/metrics.go
ingestDurationHist, err = meter.Float64Histogram(
    metrics.MetricNameWithSubsystem("knowledge", "ingest_duration_seconds"),
    // becomes "compozy_knowledge_ingest_duration_seconds"
)
```

### 3. **Benefits of this approach:**

✅ **Single source of truth** - Change prefix in one place  
✅ **Compile-time safety** - Helper function ensures prefix is always applied  
✅ **Discoverability** - Easy to find and understand the pattern  
✅ **IDE support** - Autocomplete helps developers use the helper  
✅ **Testability** - Easy to mock or override in tests  
✅ **Consistency** - Impossible to forget the prefix

### 4. **Migration Strategy:**

1. Create `engine/infra/monitoring/metrics/naming.go` with the helper functions
2. Update metrics in this order (by impact):
   - **High Priority** (missing prefix):
     - `engine/infra/monitoring/execution_metrics.go`
     - `engine/knowledge/metrics.go`
     - `engine/auth/metrics.go`
   - **Medium Priority** (already have prefix, but hardcoded):
     - All other files in `engine/infra/monitoring/`
     - Other engine packages
3. Add linter rule or test to ensure all metrics use the helper function

### 5. **Optional: Add a test to enforce the pattern:**

```go
// engine/infra/monitoring/metrics/naming_test.go
func TestMetricNaming(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"simple metric", "requests_total", "compozy_requests_total"},
        {"subsystem metric", "http_exec_latency", "compozy_http_exec_latency"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MetricName(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

This approach follows **Prometheus/OpenTelemetry best practices** and provides the best balance of consistency, maintainability, and developer experience.
