---
title: "Database Connection Pool Metrics"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_MONITORING.md"
issue_index: "2"
sequence: "13"
---

## Database Connection Pool Metrics

**Priority:** 🔴 CRITICAL

**Location:** `engine/infra/postgres/metrics.go`, `engine/infra/redis/metrics.go` (NEW FILES)

**Why Critical:**

- Cannot detect connection exhaustion
- No visibility into pool saturation
- Cannot tune pool sizes
- Missing connection leak detection

**Metrics to Add:**

```yaml
postgres_connections_open:
  type: gauge
  description: "Number of open Postgres connections"

postgres_connections_in_use:
  type: gauge
  description: "Number of Postgres connections currently in use"

postgres_connections_idle:
  type: gauge
  description: "Number of idle Postgres connections"

postgres_connection_wait_duration_seconds:
  type: histogram
  unit: seconds
  buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2]
  description: "Time spent waiting for a connection from the pool"

redis_pool_size:
  type: gauge
  description: "Configured Redis connection pool size"

redis_pool_hits_total:
  type: counter
  description: "Number of times a connection was obtained from the pool"

redis_pool_misses_total:
  type: counter
  description: "Number of times a new connection had to be created"

redis_pool_timeouts_total:
  type: counter
  description: "Number of times waiting for a connection timed out"

redis_pool_idle_conns:
  type: gauge
  description: "Number of idle Redis connections in the pool"

redis_pool_stale_conns_total:
  type: counter
  description: "Number of stale Redis connections removed"
```

**Implementation (Postgres):**

```go
// engine/infra/postgres/metrics.go (NEW FILE)
package postgres

import (
    "context"
    "database/sql"
    "sync"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    connectionsOpen     metric.Int64ObservableGauge
    connectionsInUse    metric.Int64ObservableGauge
    connectionsIdle     metric.Int64ObservableGauge
    connectionWaitTime  metric.Float64Histogram

    dbInstance *sql.DB
)

func InitMetrics(ctx context.Context, db *sql.DB) {
    dbInstance = db
    meter := otel.GetMeterProvider().Meter("compozy.postgres")

    once.Do(func() {
        var err error

        connectionsOpen, err = meter.Int64ObservableGauge(
            "compozy_postgres_connections_open",
            metric.WithDescription("Number of open Postgres connections"),
        )
        if err != nil {
            panic(err)
        }

        connectionsInUse, err = meter.Int64ObservableGauge(
            "compozy_postgres_connections_in_use",
            metric.WithDescription("Number of Postgres connections currently in use"),
        )
        if err != nil {
            panic(err)
        }

        connectionsIdle, err = meter.Int64ObservableGauge(
            "compozy_postgres_connections_idle",
            metric.WithDescription("Number of idle Postgres connections"),
        )
        if err != nil {
            panic(err)
        }

        connectionWaitTime, err = meter.Float64Histogram(
            "compozy_postgres_connection_wait_duration_seconds",
            metric.WithDescription("Time spent waiting for a connection from the pool"),
            metric.WithUnit("seconds"),
        )
        if err != nil {
            panic(err)
        }

        // Register callback to observe pool stats
        _, err = meter.RegisterCallback(
            func(_ context.Context, o metric.Observer) error {
                if dbInstance == nil {
                    return nil
                }

                stats := dbInstance.Stats()

                o.ObserveInt64(connectionsOpen, int64(stats.OpenConnections))
                o.ObserveInt64(connectionsInUse, int64(stats.InUse))
                o.ObserveInt64(connectionsIdle, int64(stats.Idle))

                return nil
            },
            connectionsOpen,
            connectionsInUse,
            connectionsIdle,
        )
        if err != nil {
            panic(err)
        }
    })
}

// RecordConnectionWait records time spent waiting for a connection
func RecordConnectionWait(ctx context.Context, duration time.Duration) {
    if connectionWaitTime != nil {
        connectionWaitTime.Record(ctx, duration.Seconds())
    }
}

// WithMetrics wraps a DB connection with metrics tracking
func WithMetrics(db *sql.DB) *sql.DB {
    // Create a wrapper that tracks connection wait time
    // This is a simplified version - in production you'd use a proper connection wrapper
    return db
}
```

**Implementation (Redis):**

```go
// engine/infra/redis/metrics.go (NEW FILE)
package redis

import (
    "context"
    "sync"

    "github.com/redis/go-redis/v9"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    poolSize        metric.Int64ObservableGauge
    poolHits        metric.Int64Counter
    poolMisses      metric.Int64Counter
    poolTimeouts    metric.Int64Counter
    poolIdleConns   metric.Int64ObservableGauge
    poolStaleConns  metric.Int64Counter

    redisClient *redis.Client
)

func InitMetrics(ctx context.Context, client *redis.Client) {
    redisClient = client
    meter := otel.GetMeterProvider().Meter("compozy.redis")

    once.Do(func() {
        var err error

        poolSize, err = meter.Int64ObservableGauge(
            "compozy_redis_pool_size",
            metric.WithDescription("Configured Redis connection pool size"),
        )
        if err != nil {
            panic(err)
        }

        poolHits, err = meter.Int64Counter(
            "compozy_redis_pool_hits_total",
            metric.WithDescription("Number of times a connection was obtained from the pool"),
        )
        if err != nil {
            panic(err)
        }

        poolMisses, err = meter.Int64Counter(
            "compozy_redis_pool_misses_total",
            metric.WithDescription("Number of times a new connection had to be created"),
        )
        if err != nil {
            panic(err)
        }

        poolTimeouts, err = meter.Int64Counter(
            "compozy_redis_pool_timeouts_total",
            metric.WithDescription("Number of times waiting for a connection timed out"),
        )
        if err != nil {
            panic(err)
        }

        poolIdleConns, err = meter.Int64ObservableGauge(
            "compozy_redis_pool_idle_conns",
            metric.WithDescription("Number of idle Redis connections in the pool"),
        )
        if err != nil {
            panic(err)
        }

        poolStaleConns, err = meter.Int64Counter(
            "compozy_redis_pool_stale_conns_total",
            metric.WithDescription("Number of stale Redis connections removed"),
        )
        if err != nil {
            panic(err)
        }

        // Register callback to observe pool stats
        _, err = meter.RegisterCallback(
            func(_ context.Context, o metric.Observer) error {
                if redisClient == nil {
                    return nil
                }

                stats := redisClient.PoolStats()

                o.ObserveInt64(poolSize, int64(stats.TotalConns))
                o.ObserveInt64(poolIdleConns, int64(stats.IdleConns))

                // Record counters (they accumulate)
                poolHits.Add(context.Background(), int64(stats.Hits))
                poolMisses.Add(context.Background(), int64(stats.Misses))
                poolTimeouts.Add(context.Background(), int64(stats.Timeouts))
                poolStaleConns.Add(context.Background(), int64(stats.StaleConns))

                return nil
            },
            poolSize,
            poolIdleConns,
        )
        if err != nil {
            panic(err)
        }
    })
}
```

**PromQL Queries:**

```promql
# Postgres connection pool utilization
compozy_postgres_connections_in_use
  / compozy_postgres_connections_open * 100

# Postgres connections near limit
compozy_postgres_connections_open
  / on() group_left() compozy_postgres_max_open_connections * 100 > 90

# Redis pool hit rate
rate(compozy_redis_pool_hits_total[5m])
  / (rate(compozy_redis_pool_hits_total[5m])
     + rate(compozy_redis_pool_misses_total[5m])) * 100

# Redis pool exhaustion
compozy_redis_pool_timeouts_total > 0

# Idle connection waste (Postgres)
compozy_postgres_connections_idle / compozy_postgres_connections_open > 0.5
```

**Alerting:**

```yaml
# prometheus/alerts.yml
groups:
  - name: database_pools
    rules:
      - alert: PostgresPoolNearLimit
        expr: |
          compozy_postgres_connections_in_use 
            / compozy_postgres_connections_open > 0.9
        for: 5m
        annotations:
          summary: "Postgres connection pool >90% utilized"

      - alert: RedisPoolExhaustion
        expr: rate(compozy_redis_pool_timeouts_total[5m]) > 0
        for: 2m
        annotations:
          summary: "Redis pool experiencing connection timeouts"
```

**Effort:** M (4h)  
**Risk:** Low - passive metrics collection
