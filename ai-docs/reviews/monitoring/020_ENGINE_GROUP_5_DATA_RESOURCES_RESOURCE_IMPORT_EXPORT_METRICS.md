---
title: "Resource Import/Export Metrics"
group: "ENGINE_GROUP_5_DATA_RESOURCES"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_5_DATA_RESOURCES_MONITORING.md"
issue_index: "3"
sequence: "20"
---

## Resource Import/Export Metrics

**Priority:** 🟡 MEDIUM

**Metrics:**

```yaml
resource_import_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - format: enum[json, yaml]
  buckets: [0.1, 0.5, 1, 5, 10, 30]
  description: "Resource import operation duration"

resource_export_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - format: enum[json, yaml]
  buckets: [0.1, 0.5, 1, 5, 10, 30]
  description: "Resource export operation duration"

resource_import_items_total:
  type: counter
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Number of resources imported"

resource_export_items_total:
  type: counter
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Number of resources exported"

resource_import_errors_total:
  type: counter
  labels:
    - error_type: enum[parse_error, validation_error, duplicate]
  description: "Resource import errors"
```

**PromQL Queries:**

```promql
# Import/export throughput
rate(resource_import_items_total[5m])
rate(resource_export_items_total[5m])

# Import duration
rate(resource_import_duration_seconds_sum[5m])
  / rate(resource_import_duration_seconds_count[5m])

# Import error rate
rate(resource_import_errors_total[5m])
```

**Effort:** S (2h)  
**Risk:** None
