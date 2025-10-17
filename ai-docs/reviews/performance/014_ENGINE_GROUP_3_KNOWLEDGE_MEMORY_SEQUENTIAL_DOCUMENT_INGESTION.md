---
title: "Sequential Document Ingestion"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_PERFORMANCE.md"
issue_index: "3"
sequence: "14"
---

## Sequential Document Ingestion

**Location:** `engine/knowledge/ingestion/pipeline.go:89–167`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// Processes documents one at a time
func (p *Pipeline) Ingest(ctx context.Context, docs []*Document) error {
    for _, doc := range docs {
        // Sequential: chunk → embed → store
        chunks, err := p.chunker.Chunk(doc)
        if err != nil {
            return err
        }

        for _, chunk := range chunks {
            embedding, err := p.embedder.Embed(ctx, chunk.Content)
            if err != nil {
                return err
            }

            if err := p.store.Save(ctx, chunk, embedding); err != nil {
                return err
            }
        }
    }
    return nil
}
```

**Problem:** For 100 documents × 10 chunks each = 1000 sequential operations

**Fix - Batch Processing with Worker Pool:**

```go
// engine/knowledge/ingestion/batch_pipeline.go (NEW)
package ingestion

import (
    "context"
    "sync"
)

type BatchPipeline struct {
    chunker   Chunker
    embedder  Embedder
    store     Store
    batchSize int
    workers   int
}

type chunkJob struct {
    docID   string
    chunk   *Chunk
    result  chan<- *embeddedChunk
    errChan chan<- error
}

type embeddedChunk struct {
    chunk     *Chunk
    embedding []float32
}

func (p *BatchPipeline) Ingest(ctx context.Context, docs []*Document) error {
    // Stage 1: Parallel chunking
    chunks := make([]*Chunk, 0, len(docs)*10)
    chunksMu := sync.Mutex{}

    var wg sync.WaitGroup
    for _, doc := range docs {
        wg.Add(1)
        go func(d *Document) {
            defer wg.Done()

            docChunks, err := p.chunker.Chunk(d)
            if err != nil {
                logger.FromContext(ctx).Error("chunking failed", "doc", d.ID, "error", err)
                return
            }

            chunksMu.Lock()
            chunks = append(chunks, docChunks...)
            chunksMu.Unlock()
        }(doc)
    }
    wg.Wait()

    // Stage 2: Batch embedding
    embeddings := make([][]float32, len(chunks))

    for i := 0; i < len(chunks); i += p.batchSize {
        end := i + p.batchSize
        if end > len(chunks) {
            end = len(chunks)
        }

        batch := chunks[i:end]
        texts := make([]string, len(batch))
        for j, chunk := range batch {
            texts[j] = chunk.Content
        }

        // Batch embed API call
        batchEmbeddings, err := p.embedder.EmbedBatch(ctx, texts)
        if err != nil {
            return fmt.Errorf("batch embedding failed: %w", err)
        }

        copy(embeddings[i:end], batchEmbeddings)
    }

    // Stage 3: Batch insert to database
    return p.store.SaveBatch(ctx, chunks, embeddings)
}

// Add batch methods to interfaces:
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type Store interface {
    Save(ctx context.Context, chunk *Chunk, embedding []float32) error
    SaveBatch(ctx context.Context, chunks []*Chunk, embeddings [][]float32) error
}

// In pgvector store:
func (s *Store) SaveBatch(ctx context.Context, chunks []*Chunk, embeddings [][]float32) error {
    // Use COPY for bulk insert (much faster than INSERT)
    _, err := s.pool.CopyFrom(
        ctx,
        pgx.Identifier{"knowledge_vectors"},
        []string{"id", "content", "metadata", "embedding"},
        pgx.CopyFromSlice(len(chunks), func(i int) ([]any, error) {
            return []any{
                chunks[i].ID,
                chunks[i].Content,
                chunks[i].Metadata,
                pgvector.NewVector(embeddings[i]),
            }, nil
        }),
    )
    return err
}
```

**Impact:**

- 100 documents: 30s → 5s (6x faster)
- Utilizes embedding API batch endpoints
- Reduces database round-trips

**Effort:** L (5-6h)  
**Risk:** Medium - error handling more complex

## Medium Priority Issues
