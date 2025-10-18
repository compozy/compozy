# Engine Group 6: Project Management - Monitoring Improvements

**Packages:** project

---

## Executive Summary

Comprehensive monitoring for project configuration management.

**Current State:**

- ❌ No project load/validation metrics
- ❌ No config parse timing
- ❌ No indexing duration tracking
- ❌ No project lifecycle events

---

## Missing Metrics

### 1. Project Load and Validation Metrics

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

---

### 2. Config Parse and Cache Metrics

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

---

### 3. Indexing Duration and Item Counts

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

---

### 4. Project Lifecycle Events

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

---

## Implementation Priorities

### Phase 1: Critical Observability (Week 1)

1. Project load/validation metrics (#1) - **3h**
2. Indexing metrics (#3) - **3h**

### Phase 2: Caching & Lifecycle (Week 2)

3. Config parse/cache metrics (#2) - **2h**
4. Lifecycle events (#4) - **2h**

**Total effort:** 10 hours

---

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

---

## Related Documentation

- **GROUP_6_PERFORMANCE.md** - Project management performance optimizations
- **GROUP_5_MONITORING.md** - Data & resource monitoring patterns
