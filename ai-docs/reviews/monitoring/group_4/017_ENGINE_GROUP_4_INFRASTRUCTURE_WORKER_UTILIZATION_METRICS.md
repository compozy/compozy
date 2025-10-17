---
title: "Worker Utilization Metrics"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_MONITORING.md"
issue_index: "6"
sequence: "17"
---

## Worker Utilization Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/worker/metrics.go` (NEW FILE)

**Metrics to Add:**

```yaml
worker_activities_executing:
  type: gauge
  description: "Number of activities currently executing"

worker_workflows_executing:
  type: gauge
  description: "Number of workflows currently executing"

worker_task_queue_depth:
  type: gauge
  labels:
    - queue_name: string
  description: "Number of tasks waiting in queue"

worker_activity_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - activity_type: string
    - outcome: enum[success, error, timeout]
  buckets: [0.1, 0.5, 1, 5, 10, 30, 60, 300]
  description: "Activity execution duration"

worker_utilization_ratio:
  type: gauge
  labels:
    - worker_type: enum[activity, workflow]
  description: "Worker utilization (executing / max_concurrent)"
```

**PromQL Queries:**

```promql
# Worker utilization percentage
worker_activities_executing / worker_max_concurrent_activities * 100

# Queue backlog alert
worker_task_queue_depth > 100

# Average activity duration
rate(worker_activity_duration_seconds_sum[5m])
  / rate(worker_activity_duration_seconds_count[5m])
```

**Effort:** M (3h)  
**Risk:** Low

## Implementation Priorities

### Phase 1: Critical Observability (Week 1)

1. HTTP request/response metrics (#1) - **3h**
2. Database pool metrics (#2) - **4h**

### Phase 2: Security & Webhook (Week 2)

3. Auth failure tracking (#3) - **3h**
4. Webhook metrics (#4) - **2h**

### Phase 3: Operations Metrics (Week 3)

5. Autoload metrics (#5) - **2h**
6. Worker utilization (#6) - **3h**

**Total effort:** 17 hours

## Dashboards

### HTTP Performance Dashboard

```yaml
panels:
  - title: Request Rate
    query: rate(compozy_http_requests_total[5m])

  - title: P95 Latency
    query: histogram_quantile(0.95, rate(compozy_http_request_duration_seconds_bucket[5m]))

  - title: Request Size Distribution
    query: rate(compozy_http_request_size_bytes_bucket[5m])

  - title: In-Flight Requests
    query: compozy_http_requests_in_flight
```

### Database Health Dashboard

```yaml
panels:
  - title: Postgres Pool Utilization
    query: compozy_postgres_connections_in_use / compozy_postgres_connections_open * 100

  - title: Redis Pool Hit Rate
    query: rate(compozy_redis_pool_hits_total[5m]) / (rate(compozy_redis_pool_hits_total[5m]) + rate(compozy_redis_pool_misses_total[5m])) * 100
```

## Related Documentation

- **GROUP_4_PERFORMANCE.md** - Infrastructure performance optimizations
- **GROUP_1_MONITORING.md** - Runtime execution monitoring patterns
- **GROUP_2_MONITORING.md** - LLM and MCP monitoring patterns
