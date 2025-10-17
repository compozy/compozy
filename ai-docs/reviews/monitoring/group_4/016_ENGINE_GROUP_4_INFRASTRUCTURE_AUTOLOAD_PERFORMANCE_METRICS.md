---
title: "Autoload Performance Metrics"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "monitoring"
priority: "🟢 LOW"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_MONITORING.md"
issue_index: "5"
sequence: "16"
---

## Autoload Performance Metrics

**Priority:** 🟢 LOW

**Location:** `engine/autoload/metrics.go` (NEW FILE)

**Metrics to Add:**

```yaml
autoload_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - project: string
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30]
  description: "Time to complete autoload process"

autoload_files_processed_total:
  type: counter
  labels:
    - project: string
    - outcome: enum[success, error]
  description: "Total files processed by autoload"

autoload_configs_loaded_total:
  type: counter
  labels:
    - project: string
    - type: enum[workflow, agent, tool, mcp]
  description: "Total configurations loaded by type"

autoload_errors_total:
  type: counter
  labels:
    - project: string
    - error_type: enum[parse_error, validation_error, duplicate_error, security_error]
  description: "Total autoload errors by category"
```

**Implementation:** (See autoload.go, add metrics to Load() and LoadWithResult())

**PromQL Queries:**

```promql
# Autoload duration trend
rate(autoload_duration_seconds_sum[5m])
  / rate(autoload_duration_seconds_count[5m])

# Files per second processing rate
rate(autoload_files_processed_total[5m])

# Error rate by type
sum by (error_type) (rate(autoload_errors_total[5m]))
```

**Effort:** S (2h)  
**Risk:** None
