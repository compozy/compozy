---
title: "Resource Operation Metrics"
group: "ENGINE_GROUP_5_DATA_RESOURCES"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_5_DATA_RESOURCES_MONITORING.md"
issue_index: "2"
sequence: "19"
---

## Resource Operation Metrics

**Priority:** 🔴 CRITICAL

**Metrics:**

```yaml
resource_operations_total:
  type: counter
  labels:
    - operation: enum[get, put, delete, list]
    - resource_type: enum[agent, tool, schema, model, memory]
    - outcome: enum[success, error]
  description: "Resource store operations"

resource_operation_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - operation: enum[get, put, delete, list]
    - resource_type: enum[agent, tool, schema, model, memory]
  buckets: [0.0001, 0.001, 0.01, 0.1, 1]
  description: "Resource operation latency"

resource_store_size:
  type: gauge
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Number of resources stored by type"

resource_etag_mismatches_total:
  type: counter
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Optimistic locking conflicts (ETag mismatches)"
```

**PromQL Queries:**

```promql
# Operation latency by type
histogram_quantile(0.95,
  rate(resource_operation_duration_seconds_bucket[5m])) by (operation, resource_type)

# Operations per second
rate(resource_operations_total[5m])

# Error rate
rate(resource_operations_total{outcome="error"}[5m])
  / rate(resource_operations_total[5m])

# ETag conflict rate
rate(resource_etag_mismatches_total[5m])
```

**Effort:** M (4h)  
**Risk:** Low
