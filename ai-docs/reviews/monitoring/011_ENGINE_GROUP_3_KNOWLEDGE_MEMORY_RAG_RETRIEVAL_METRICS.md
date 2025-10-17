---
title: "RAG Retrieval Metrics"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_MONITORING.md"
issue_index: "4"
sequence: "11"
---

## RAG Retrieval Metrics

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
      **Document Version:** 1.0  
       **Last Updated:** 2025-01-16  
       **Owner:** Observability Team
