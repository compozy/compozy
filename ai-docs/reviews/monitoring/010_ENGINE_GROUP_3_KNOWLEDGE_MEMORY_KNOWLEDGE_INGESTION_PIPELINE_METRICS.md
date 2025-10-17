---
title: "Knowledge Ingestion Pipeline Metrics"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_MONITORING.md"
issue_index: "3"
sequence: "10"
---

## Knowledge Ingestion Pipeline Metrics

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
