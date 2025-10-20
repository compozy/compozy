# Engine Group 3: Knowledge & Memory - Monitoring Improvements

**Packages:** knowledge (ingestion, retrieval, chunking), memory (store, vector, embeddings), rag (pipeline, retrieval)

---

## Executive Summary

Monitoring instrumentation for knowledge ingestion pipelines, vector similarity search, embedding generation, and RAG operations. Critical gaps in vector store performance and embedding API tracking.

**Current State:**

- ✅ Basic HTTP endpoint metrics exist
- ❌ No vector similarity search metrics
- ❌ No embedding generation tracking
- ❌ No ingestion pipeline metrics
- ❌ No cache hit rate monitoring

---

## Missing Metrics

### 1. Vector Similarity Search Metrics

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

---

### 2. Embedding Generation Metrics

**Priority:** 🔴 HIGH

**Location:** `engine/memory/embeddings/`

**Metrics to Add:**

```yaml
embeddings_generate_seconds:
  type: histogram
  unit: seconds
  labels:
    - provider: string (openai, cohere)
    - model: string
    - batch_size: int
  buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5]
  description: "Embedding generation latency"

embeddings_tokens_total:
  type: counter
  labels:
    - provider: string
    - model: string
  description: "Total tokens processed for embeddings"

embeddings_cache_hits_total:
  type: counter
  labels:
    - provider: string
  description: "Embedding cache hits"

embeddings_cache_misses_total:
  type: counter
  labels:
    - provider: string
  description: "Embedding cache misses"

embeddings_errors_total:
  type: counter
  labels:
    - provider: string
    - error_type: enum[auth, rate_limit, invalid_input, server_error]
  description: "Embedding generation errors"
```

**Implementation:**

```go
// engine/memory/embeddings/metrics.go (NEW)
package embeddings

import (
    "context"
    "sync"
    "time"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    generateLatency metric.Float64Histogram
    tokensCounter   metric.Int64Counter
    cacheHits       metric.Int64Counter
    cacheMisses     metric.Int64Counter
    errorsCounter   metric.Int64Counter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.embeddings")

    generateLatency, _ = meter.Float64Histogram(
        "embeddings_generate_seconds",
        metric.WithDescription("Embedding generation latency"),
        metric.WithUnit("s"),
    )

    tokensCounter, _ = meter.Int64Counter(
        "embeddings_tokens_total",
        metric.WithDescription("Tokens processed"),
    )

    cacheHits, _ = meter.Int64Counter(
        "embeddings_cache_hits_total",
        metric.WithDescription("Cache hits"),
    )

    cacheMisses, _ = meter.Int64Counter(
        "embeddings_cache_misses_total",
        metric.WithDescription("Cache misses"),
    )

    errorsCounter, _ = meter.Int64Counter(
        "embeddings_errors_total",
        metric.WithDescription("Generation errors"),
    )
}

func RecordGeneration(ctx context.Context, provider, model string, batchSize int, duration time.Duration, tokenCount int) {
    once.Do(initMetrics)

    generateLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.Int("batch_size", batchSize),
        ))

    tokensCounter.Add(ctx, int64(tokenCount),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
        ))
}

func RecordCacheHit(ctx context.Context, provider string) {
    once.Do(initMetrics)
    cacheHits.Add(ctx, 1, metric.WithAttributes(attribute.String("provider", provider)))
}

func RecordCacheMiss(ctx context.Context, provider string) {
    once.Do(initMetrics)
    cacheMisses.Add(ctx, 1, metric.WithAttributes(attribute.String("provider", provider)))
}

func RecordError(ctx context.Context, provider, errorType string) {
    once.Do(initMetrics)

    errorsCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("error_type", errorType),
        ))
}
```

**Usage:**

```go
// In cached_embedder.go
func (e *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    key := hashText(text)

    if embedding, ok := e.cache.Get(key); ok {
        metrics.RecordCacheHit(ctx, e.provider)
        return embedding, nil
    }

    metrics.RecordCacheMiss(ctx, e.provider)

    start := time.Now()
    embedding, tokenCount, err := e.underlying.Embed(ctx, text)
    if err != nil {
        metrics.RecordError(ctx, e.provider, categorizeError(err))
        return nil, err
    }

    metrics.RecordGeneration(ctx, e.provider, e.model, 1, time.Since(start), tokenCount)

    e.cache.Add(key, embedding)
    return embedding, nil
}
```

**Dashboard Queries:**

```promql
# Embedding latency by batch size
histogram_quantile(0.95,
  rate(embeddings_generate_seconds_bucket[5m])
) by (batch_size)

# Cache hit rate
rate(embeddings_cache_hits_total[5m]) /
(rate(embeddings_cache_hits_total[5m]) + rate(embeddings_cache_misses_total[5m]))

# Token usage rate
rate(embeddings_tokens_total[5m]) by (provider, model)
```

**Effort:** M (2-3h)

---

### 3. Knowledge Ingestion Pipeline Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/knowledge/ingestion/pipeline.go`

**Metrics to Add:**

```yaml
ingestion_pipeline_seconds:
  type: histogram
  unit: seconds
  labels:
    - stage: enum[chunking, embedding, storage]
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30, 60, 120]
  description: "Ingestion pipeline stage duration"

ingestion_documents_total:
  type: counter
  labels:
    - outcome: enum[success, error]
  description: "Documents processed through ingestion"

ingestion_chunks_total:
  type: counter
  description: "Total chunks generated from documents"

ingestion_batch_size:
  type: histogram
  buckets: [1, 5, 10, 25, 50, 100, 200]
  description: "Documents per ingestion batch"

ingestion_errors_total:
  type: counter
  labels:
    - stage: enum[chunking, embedding, storage]
    - error_type: string
  description: "Ingestion errors by stage"
```

**Implementation:**

```go
// engine/knowledge/ingestion/metrics.go (NEW)
package ingestion

func RecordPipelineStage(ctx context.Context, stage string, duration time.Duration) {
    once.Do(initMetrics)

    pipelineLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(attribute.String("stage", stage)))
}

func RecordDocuments(ctx context.Context, count int, outcome string) {
    once.Do(initMetrics)

    documentsCounter.Add(ctx, int64(count),
        metric.WithAttributes(attribute.String("outcome", outcome)))
}

func RecordChunks(ctx context.Context, count int) {
    once.Do(initMetrics)
    chunksCounter.Add(ctx, int64(count))
}

func RecordBatchSize(ctx context.Context, size int) {
    once.Do(initMetrics)
    batchSize.Record(ctx, float64(size))
}

func RecordError(ctx context.Context, stage, errorType string) {
    once.Do(initMetrics)

    errorsCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("stage", stage),
            attribute.String("error_type", errorType),
        ))
}
```

**Usage:**

```go
// In batch_pipeline.go
func (p *BatchPipeline) Ingest(ctx context.Context, docs []*Document) error {
    metrics.RecordBatchSize(ctx, len(docs))

    // Stage 1: Chunking
    chunkStart := time.Now()
    chunks, err := p.chunkDocuments(ctx, docs)
    if err != nil {
        metrics.RecordError(ctx, "chunking", categorizeError(err))
        return err
    }
    metrics.RecordPipelineStage(ctx, "chunking", time.Since(chunkStart))
    metrics.RecordChunks(ctx, len(chunks))

    // Stage 2: Embedding
    embedStart := time.Now()
    embeddings, err := p.embedChunks(ctx, chunks)
    if err != nil {
        metrics.RecordError(ctx, "embedding", categorizeError(err))
        return err
    }
    metrics.RecordPipelineStage(ctx, "embedding", time.Since(embedStart))

    // Stage 3: Storage
    storeStart := time.Now()
    if err := p.store.SaveBatch(ctx, chunks, embeddings); err != nil {
        metrics.RecordError(ctx, "storage", categorizeError(err))
        metrics.RecordDocuments(ctx, len(docs), "error")
        return err
    }
    metrics.RecordPipelineStage(ctx, "storage", time.Since(storeStart))

    metrics.RecordDocuments(ctx, len(docs), "success")
    return nil
}
```

**Dashboard Queries:**

```promql
# Pipeline bottleneck analysis
histogram_quantile(0.95,
  rate(ingestion_pipeline_seconds_bucket[5m])
) by (stage)

# Ingestion throughput
rate(ingestion_documents_total{outcome="success"}[5m])

# Chunks per document ratio
rate(ingestion_chunks_total[5m]) /
rate(ingestion_documents_total[5m])
```

**Effort:** M (2h)

---

### 4. RAG Retrieval Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/rag/retrieval/retriever.go`

**Metrics to Add:**

```yaml
rag_retrieval_seconds:
  type: histogram
  unit: seconds
  labels:
    - strategy: enum[similarity, hybrid, keyword]
  buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2]
  description: "RAG retrieval latency"

rag_context_size_bytes:
  type: histogram
  buckets: [100, 500, 1000, 5000, 10000, 50000, 100000]
  description: "Context size passed to LLM"

rag_retrieval_relevance_score:
  type: histogram
  buckets: [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0]
  description: "Average relevance score of retrieved chunks"
```

**Effort:** S (1-2h)

---

## OpenTelemetry Spans

### Vector Search Span

```go
func (c *Client) SimilaritySearch(ctx context.Context, embedding []float32, topK int) ([]*Result, error) {
    tracer := otel.Tracer("compozy.vector")
    ctx, span := tracer.Start(ctx, "vector.similarity_search")
    defer span.End()

    span.SetAttributes(
        attribute.String("index_type", c.indexType),
        attribute.Int("top_k", topK),
        attribute.Int("embedding_dimensions", len(embedding)),
    )

    results, err := c.doSearch(ctx, embedding, topK)

    if err != nil {
        span.RecordError(err)
    } else {
        span.SetAttributes(
            attribute.Int("results_count", len(results)),
            attribute.Float64("min_distance", results[0].Distance),
        )
    }

    return results, err
}
```

### Embedding Generation Span

```go
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
    tracer := otel.Tracer("compozy.embeddings")
    ctx, span := tracer.Start(ctx, "embeddings.generate")
    defer span.End()

    span.SetAttributes(
        attribute.String("provider", e.provider),
        attribute.String("model", e.model),
        attribute.Int("text_length", len(text)),
    )

    embedding, err := e.doEmbed(ctx, text)

    if err != nil {
        span.RecordError(err)
    }

    return embedding, err
}
```

### Ingestion Pipeline Span

```go
func (p *Pipeline) Ingest(ctx context.Context, docs []*Document) error {
    tracer := otel.Tracer("compozy.ingestion")
    ctx, span := tracer.Start(ctx, "ingestion.batch")
    defer span.End()

    span.SetAttributes(
        attribute.Int("document_count", len(docs)),
    )

    // Child spans for each stage
    ctx, chunkSpan := tracer.Start(ctx, "ingestion.chunking")
    chunks, err := p.chunk(ctx, docs)
    chunkSpan.SetAttributes(attribute.Int("chunks_generated", len(chunks)))
    chunkSpan.End()

    ctx, embedSpan := tracer.Start(ctx, "ingestion.embedding")
    embeddings, err := p.embed(ctx, chunks)
    embedSpan.End()

    ctx, storeSpan := tracer.Start(ctx, "ingestion.storage")
    err = p.store(ctx, chunks, embeddings)
    storeSpan.End()

    if err != nil {
        span.RecordError(err)
    }

    return err
}
```

**Effort:** M (2h for all spans)

---

## Dashboard Layout

### Knowledge & Vector Store Dashboard

```
┌─────────────────────────────────────────────┐
│ Vector Search Latency p95 by topK         │
│ [Line chart]                                │
└─────────────────────────────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Search Result Quality│ Connection Pool      │
│ [Distance heatmap]   │ [Gauge]              │
└──────────────────────┴──────────────────────┘

┌──────────────────────────────────────────────┐
│ Embedding Generation Latency by Batch Size │
│ [Grouped bar chart]                         │
└──────────────────────────────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Cache Hit Rate       │ Token Usage          │
│ [Single stat + trend]│ [Counter]            │
└──────────────────────┴──────────────────────┘
```

### Ingestion Pipeline Dashboard

```
┌─────────────────────────────────────────────┐
│ Pipeline Stage Duration Breakdown          │
│ [Stacked bar: chunking, embedding, storage]│
└─────────────────────────────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Ingestion Throughput │ Chunks per Document  │
│ [Line chart]         │ [Histogram]          │
└──────────────────────┴──────────────────────┘

┌──────────────────────────────────────────────┐
│ Error Rate by Stage                         │
│ [Stacked area]                              │
└──────────────────────────────────────────────┘
```

---

## Alert Rules

```yaml
groups:
  - name: vector_store
    interval: 30s
    rules:
      - alert: SlowVectorSearch
        expr: |
          histogram_quantile(0.95,
            rate(vector_similarity_search_seconds_bucket[5m])
          ) > 1
        for: 5m
        annotations:
          summary: "Vector search p95 latency > 1s"

      - alert: PoorSearchRelevance
        expr: |
          avg(vector_similarity_distance_min) > 0.7
        for: 10m
        annotations:
          summary: "Average min distance > 0.7 (poor relevance)"

      - alert: ConnectionPoolExhausted
        expr: |
          vector_store_connections_active >= 20
        for: 2m
        annotations:
          summary: "Vector store connection pool at maximum"

  - name: embeddings
    interval: 30s
    rules:
      - alert: HighEmbeddingLatency
        expr: |
          histogram_quantile(0.95,
            rate(embeddings_generate_seconds_bucket[5m])
          ) > 2
        for: 5m
        annotations:
          summary: "Embedding generation p95 > 2s"

      - alert: LowCacheHitRate
        expr: |
          rate(embeddings_cache_hits_total[5m]) /
          (rate(embeddings_cache_hits_total[5m]) + rate(embeddings_cache_misses_total[5m])) < 0.2
        for: 10m
        annotations:
          summary: "Embedding cache hit rate < 20%"

  - name: ingestion
    interval: 30s
    rules:
      - alert: HighIngestionErrorRate
        expr: |
          rate(ingestion_errors_total[5m]) > 0.1
        for: 2m
        annotations:
          summary: "Ingestion error rate > 0.1/sec"

      - alert: SlowIngestion
        expr: |
          histogram_quantile(0.95,
            rate(ingestion_pipeline_seconds_bucket{stage="embedding"}[5m])
          ) > 30
        for: 5m
        annotations:
          summary: "Embedding stage taking > 30s"
```

---

## Implementation Plan

### Week 1 - Vector Store Metrics

- [ ] Create vector/metrics package
- [ ] Instrument SimilaritySearch
- [ ] Add connection tracking
- [ ] Deploy and verify

### Week 2 - Embedding & Ingestion

- [ ] Create embeddings/metrics package
- [ ] Add cache hit/miss tracking
- [ ] Instrument ingestion pipeline
- [ ] Add stage-level metrics

### Week 3 - Dashboards & Alerts

- [ ] Build Grafana dashboards
- [ ] Configure alert rules
- [ ] Add OpenTelemetry spans
- [ ] Document troubleshooting guides

---

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Observability Team
