---
title: "Project Lifecycle Events"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_MONITORING.md"
issue_index: "4"
sequence: "25"
---

## Project Lifecycle Events

**Priority:** 🟡 MEDIUM

**Metrics:**

```yaml
project_lifecycle_events_total:
  type: counter
  labels:
    - project_name: string
    - event_type: enum[created, updated, deleted, loaded, validated]
  description: "Project lifecycle events"

project_upsert_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - project_name: string
  buckets: [0.1, 0.5, 1, 5, 10]
  description: "Project upsert (create/update) duration"

project_active_count:
  type: gauge
  description: "Number of currently loaded projects"

project_last_loaded_timestamp:
  type: gauge
  labels:
    - project_name: string
  description: "Unix timestamp of last project load"
```

**PromQL Queries:**

```promql
# Projects updated per minute
rate(project_lifecycle_events_total{event_type="updated"}[1m])

# Active projects
project_active_count

# Time since last load
time() - project_last_loaded_timestamp

# Upsert duration
histogram_quantile(0.95, rate(project_upsert_duration_seconds_bucket[5m]))
```

**Effort:** S (2h)  
**Risk:** None

## Implementation Priorities

### Phase 1: Critical Observability (Week 1)

1. Project load/validation metrics (#1) - **3h**
2. Indexing metrics (#3) - **3h**

### Phase 2: Caching & Lifecycle (Week 2)

3. Config parse/cache metrics (#2) - **2h**
4. Lifecycle events (#4) - **2h**

**Total effort:** 10 hours

## Dashboards

### Project Management Dashboard

```yaml
panels:
  - title: Project Load Latency
    query: histogram_quantile(0.95, rate(project_load_duration_seconds_bucket[5m]))

  - title: Load Success Rate
    query: rate(project_loads_total{outcome="success"}[5m]) / rate(project_loads_total[5m]) * 100

  - title: Config Cache Hit Rate
    query: rate(project_config_cache_hits_total[5m]) / (rate(project_config_cache_hits_total[5m]) + rate(project_config_cache_misses_total[5m])) * 100

  - title: Indexing Duration
    query: histogram_quantile(0.95, rate(project_indexing_duration_seconds_bucket[5m]))

  - title: Resource Counts
    query: sum by (resource_type) (project_resource_count)
```

## Related Documentation

- **GROUP_6_PERFORMANCE.md** - Project management performance optimizations
- **GROUP_5_MONITORING.md** - Data & resource monitoring patterns
