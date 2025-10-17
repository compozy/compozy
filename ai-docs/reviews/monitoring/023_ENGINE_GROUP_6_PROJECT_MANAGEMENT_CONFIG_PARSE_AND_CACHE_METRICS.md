---
title: "Config Parse and Cache Metrics"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_MONITORING.md"
issue_index: "2"
sequence: "23"
---

## Config Parse and Cache Metrics

**Priority:** 🟡 MEDIUM

**Metrics:**

```yaml
project_config_parse_duration_seconds:
  type: histogram
  unit: seconds
  buckets: [0.001, 0.01, 0.05, 0.1, 0.5]
  description: "Configuration file parsing duration"

project_config_cache_hits_total:
  type: counter
  labels:
    - project_name: string
  description: "Configuration cache hits"

project_config_cache_misses_total:
  type: counter
  labels:
    - project_name: string
  description: "Configuration cache misses"

project_config_size_bytes:
  type: histogram
  unit: bytes
  labels:
    - project_name: string
  buckets: [1000, 10000, 100000, 1000000]
  description: "Configuration file size"
```

**PromQL Queries:**

```promql
# Cache hit rate
rate(project_config_cache_hits_total[5m])
  / (rate(project_config_cache_hits_total[5m]) + rate(project_config_cache_misses_total[5m])) * 100

# Parse duration
rate(project_config_parse_duration_seconds_sum[5m])
  / rate(project_config_parse_duration_seconds_count[5m])

# Large configs
histogram_quantile(0.95, rate(project_config_size_bytes_bucket[5m]))
```

**Effort:** S (2h)  
**Risk:** None
