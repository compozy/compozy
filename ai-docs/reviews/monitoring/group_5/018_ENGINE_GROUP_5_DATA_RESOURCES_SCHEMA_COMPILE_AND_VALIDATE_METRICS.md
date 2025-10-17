---
title: "Schema Compile and Validate Metrics"
group: "ENGINE_GROUP_5_DATA_RESOURCES"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_5_DATA_RESOURCES_MONITORING.md"
issue_index: "1"
sequence: "18"
---

## Schema Compile and Validate Metrics

**Priority:** 🔴 CRITICAL

**Metrics:**

```yaml
schema_compiles_total:
  type: counter
  labels:
    - cache_hit: bool
  description: "Total schema compilation attempts"

schema_compile_cache_hits_total:
  type: counter
  description: "Schema compilation cache hits"

schema_validations_total:
  type: counter
  labels:
    - outcome: enum[valid, invalid]
  description: "Schema validations performed"

schema_compile_duration_seconds:
  type: histogram
  unit: seconds
  buckets: [0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1]
  description: "Schema compilation duration"

schema_validate_duration_seconds:
  type: histogram
  unit: seconds
  buckets: [0.00001, 0.0001, 0.0005, 0.001, 0.005, 0.01]
  description: "Schema validation duration"

schema_cache_size:
  type: gauge
  description: "Number of compiled schemas in cache"
```

**PromQL Queries:**

```promql
# Cache hit rate
rate(schema_compile_cache_hits_total[5m]) / rate(schema_compiles_total[5m]) * 100

# P95 compilation time
histogram_quantile(0.95, rate(schema_compile_duration_seconds_bucket[5m]))

# Validation throughput
rate(schema_validations_total[5m])

# Invalid schema rate
rate(schema_validations_total{outcome="invalid"}[5m]) / rate(schema_validations_total[5m])
```

**Effort:** M (3h)  
**Risk:** Low
