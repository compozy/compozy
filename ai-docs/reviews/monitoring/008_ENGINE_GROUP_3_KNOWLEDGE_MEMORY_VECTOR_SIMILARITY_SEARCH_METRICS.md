---
title: "Vector Similarity Search Metrics"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_MONITORING.md"
issue_index: "1"
sequence: "8"
---

## Vector Similarity Search Metrics

**Priority:** 🔴 CRITICAL

**Location:** `engine/memory/vector/pgvector_client.go`

**Why Critical:**

- Vector search is core RAG operation
- No visibility into query performance or result quality
- Cannot detect index degradation

**Metrics to Add:**

```yaml
vector_similarity_search_seconds:
  type: histogram
  unit: seconds
  labels:
    - index_type: enum[hnsw, ivfflat]
    - top_k: int
  buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2]
  description: "Vector similarity search latency"

vector_similarity_results_total:
  type: histogram
  labels:
    - top_k: int
  buckets: [1, 5, 10, 25, 50, 100, 200]
  description: "Number of results returned per search"

vector_similarity_distance_min:
  type: histogram
  labels:
    - index_type: string
  buckets: [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0]
  description: "Minimum distance of top result (relevance indicator)"

vector_store_connections_active:
  type: gauge
  description: "Active PGVector database connections"

vector_store_errors_total:
  type: counter
  labels:
    - operation: enum[search, insert, update, delete]
    - error_type: enum[connection, timeout, query, constraint]
  description: "Vector store operation errors"
```

**Implementation:**

```go
// engine/memory/vector/metrics.go (NEW)
package vector

import (
    "context"
    "sync"
    "time"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    searchLatency     metric.Float64Histogram
    resultsCount      metric.Float64Histogram
    minDistance       metric.Float64Histogram
    activeConnections metric.Int64UpDownCounter
    errorsCounter     metric.Int64Counter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.vector")

    searchLatency, _ = meter.Float64Histogram(
        "vector_similarity_search_seconds",
        metric.WithDescription("Vector similarity search latency"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2),
    )

    resultsCount, _ = meter.Float64Histogram(
        "vector_similarity_results_total",
        metric.WithDescription("Results returned per search"),
        metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 200),
    )

    minDistance, _ = meter.Float64Histogram(
        "vector_similarity_distance_min",
        metric.WithDescription("Minimum distance of top result"),
        metric.WithExplicitBucketBoundaries(0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0),
    )

    activeConnections, _ = meter.Int64UpDownCounter(
        "vector_store_connections_active",
        metric.WithDescription("Active database connections"),
    )

    errorsCounter, _ = meter.Int64Counter(
        "vector_store_errors_total",
        metric.WithDescription("Vector store errors"),
    )
}

func RecordSearch(ctx context.Context, indexType string, topK int, duration time.Duration, resultCount int, minDist float64) {
    once.Do(initMetrics)

    searchLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("index_type", indexType),
            attribute.Int("top_k", topK),
        ))

    resultsCount.Record(ctx, float64(resultCount),
        metric.WithAttributes(attribute.Int("top_k", topK)))

    if resultCount > 0 {
        minDistance.Record(ctx, minDist,
            metric.WithAttributes(attribute.String("index_type", indexType)))
    }
}

func RecordConnection(ctx context.Context, delta int) {
    once.Do(initMetrics)
    activeConnections.Add(ctx, int64(delta))
}

func RecordError(ctx context.Context, operation, errorType string) {
    once.Do(initMetrics)

    errorsCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("operation", operation),
            attribute.String("error_type", errorType),
        ))
}
```

**Usage:**

```go
// In pgvector_client.go
func (c *Client) SimilaritySearch(ctx context.Context, embedding []float32, topK int) ([]*Result, error) {
    start := time.Now()

    query := `
        SELECT id, content, metadata, embedding <=> $1 AS distance
        FROM knowledge_vectors
        ORDER BY embedding <=> $1
        LIMIT $2
    `

    rows, err := c.pool.Query(ctx, query, pgvector.NewVector(embedding), topK)
    if err != nil {
        metrics.RecordError(ctx, "search", categorizeError(err))
        return nil, err
    }
    defer rows.Close()

    results := make([]*Result, 0, topK)
    for rows.Next() {
        var r Result
        if err := rows.Scan(&r.ID, &r.Content, &r.Metadata, &r.Distance); err != nil {
            return nil, err
        }
        results = append(results, &r)
    }

    duration := time.Since(start)
    minDist := 1.0
    if len(results) > 0 {
        minDist = results[0].Distance
    }

    metrics.RecordSearch(ctx, c.indexType, topK, duration, len(results), minDist)

    return results, nil
}

// Track connections from pool
func GetClient(ctx context.Context, config *Config) (*Client, error) {
    // ... pool creation

    // Track connections
    poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
        metrics.RecordConnection(ctx, 1)
        return nil
    }

    poolConfig.BeforeClose = func(conn *pgx.Conn) {
        metrics.RecordConnection(context.Background(), -1)
    }

    // ...
}
```

**Dashboard Queries:**

```promql
# Search latency p95 by topK
histogram_quantile(0.95,
  rate(vector_similarity_search_seconds_bucket[5m])
) by (top_k)

# Result quality - lower distance = more relevant
avg(vector_similarity_distance_min) by (index_type)

# Connection pool utilization
vector_store_connections_active

# Error rate
rate(vector_store_errors_total[5m]) by (operation, error_type)
```

**Effort:** M (3h)
