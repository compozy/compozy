---
title: "Indexing Duration and Item Counts"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_MONITORING.md"
issue_index: "3"
sequence: "24"
---

## Indexing Duration and Item Counts

**Priority:** 🔴 CRITICAL

**Metrics:**

```yaml
project_indexing_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - project_name: string
  buckets: [0.01, 0.1, 0.5, 1, 5, 10]
  description: "Project resource indexing duration"

project_indexed_resources_total:
  type: counter
  labels:
    - project_name: string
    - resource_type: enum[tool, memory, schema, model, embedder, vectordb, knowledgebase]
  description: "Total resources indexed"

project_indexing_errors_total:
  type: counter
  labels:
    - project_name: string
    - resource_type: enum[tool, memory, schema, model]
    - error_type: enum[missing_id, validation_error, store_error]
  description: "Indexing errors by type"

project_resource_count:
  type: gauge
  labels:
    - project_name: string
    - resource_type: enum[tool, memory, schema, model]
  description: "Current count of resources in project"
```

**PromQL Queries:**

```promql
# Indexing throughput
rate(project_indexed_resources_total[5m])

# Indexing duration by project
histogram_quantile(0.95, rate(project_indexing_duration_seconds_bucket[5m])) by (project_name)

# Resource counts
sum by (resource_type) (project_resource_count)

# Indexing error rate
rate(project_indexing_errors_total[5m]) / rate(project_indexed_resources_total[5m])
```

**Effort:** M (3h)  
**Risk:** Low
