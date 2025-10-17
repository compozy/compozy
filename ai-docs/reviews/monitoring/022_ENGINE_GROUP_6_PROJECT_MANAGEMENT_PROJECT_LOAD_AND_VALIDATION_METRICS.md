---
title: "Project Load and Validation Metrics"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_MONITORING.md"
issue_index: "1"
sequence: "22"
---

## Project Load and Validation Metrics

**Priority:** 🔴 CRITICAL

**Metrics:**

```yaml
project_load_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - project_name: string
    - outcome: enum[success, error]
  buckets: [0.01, 0.05, 0.1, 0.5, 1, 5]
  description: "Project configuration load duration"

project_validation_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - project_name: string
  buckets: [0.001, 0.01, 0.1, 0.5, 1]
  description: "Project validation duration"

project_load_errors_total:
  type: counter
  labels:
    - project_name: string
    - error_type: enum[parse_error, validation_error, file_not_found]
  description: "Project load errors"

project_loads_total:
  type: counter
  labels:
    - project_name: string
    - outcome: enum[success, error]
  description: "Total project load attempts"
```

**PromQL Queries:**

```promql
# Project load latency P95
histogram_quantile(0.95, rate(project_load_duration_seconds_bucket[5m]))

# Load success rate
rate(project_loads_total{outcome="success"}[5m]) / rate(project_loads_total[5m]) * 100

# Load errors by type
sum by (error_type) (rate(project_load_errors_total[5m]))

# Slowest projects to load
topk(10, rate(project_load_duration_seconds_sum[5m]) / rate(project_load_duration_seconds_count[5m]))
```

**Effort:** M (3h)  
**Risk:** Low
