# Engine Group 3: Knowledge & Memory - Performance Improvements

**Packages:** knowledge (ingestion, retrieval, chunking), memory (store, vector, embeddings), rag (pipeline, retrieval)

---

## Executive Summary

Performance improvements for knowledge ingestion, vector search, embedding generation, and RAG pipelines. Focus on connection pooling, batch operations, and query optimization.

**Priority Findings:**

- 🔴 **Critical:** Fresh PGVector client per request kills connection pool
- 🔴 **High Impact:** Vector similarity probes scale with topK parameter
- 🟡 **Medium Impact:** Sequential document ingestion under load
- 🟡 **Medium Impact:** Missing embedding result caching

---

## High Priority Issues

### 1. Fresh PGVector Client Creation Per Request

**Location:** `engine/memory/vector/pgvector_client.go:78–134`, `engine/knowledge/retrieval/retriever.go:89–145`

**Severity:** 🔴 CRITICAL

**Issue:**

```go
// Called on EVERY retrieval request
func (r *Retriever) Search(ctx context.Context, query string, topK int) ([]*Result, error) {
    // Creates new PGVector client with fresh connection
    client, err := pgvector.NewClient(ctx, r.config.DSN)
    if err != nil {
        return nil, err
    }
    defer client.Close() // Closes connection immediately after use

    embedding, err := r.embedder.Embed(ctx, query)
    if err != nil {
        return nil, err
    }

    return client.SimilaritySearch(ctx, embedding, topK)
}

// In pgvector_client.go
func NewClient(ctx context.Context, dsn string) (*Client, error) {
    // Opens fresh PostgreSQL connection
    conn, err := pgx.Connect(ctx, dsn)
    if err != nil {
        return nil, err
    }

    // Installs vector extension on each connection (expensive)
    if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
        return nil, err
    }

    return &Client{conn: conn}, nil
}
```

**Problems:**

1. Connection overhead: 10-50ms per request vs <1ms from pool
2. TLS handshake repeated every time
3. Extension check runs on every connection
4. Breaks PostgreSQL connection pooling completely
5. Under load, exhausts database connection limits

**Fix - Singleton Client with Connection Pool:**

```go
// engine/memory/vector/pgvector_client.go
package vector

import (
    "context"
    "sync"
    "github.com/jackc/pgx/v5/pgxpool"
)

var (
    clientInstance *Client
    clientOnce     sync.Once
)

type Client struct {
    pool *pgxpool.Pool
}

// Singleton initialization
func GetClient(ctx context.Context, config *Config) (*Client, error) {
    var initErr error

    clientOnce.Do(func() {
        poolConfig, err := pgxpool.ParseConfig(config.DSN)
        if err != nil {
            initErr = err
            return
        }

        // Configure pool
        poolConfig.MaxConns = 20
        poolConfig.MinConns = 2
        poolConfig.MaxConnLifetime = 1 * time.Hour
        poolConfig.MaxConnIdleTime = 30 * time.Minute
        poolConfig.HealthCheckPeriod = 1 * time.Minute

        // Connection initialization hook
        poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
            // Install vector extension once per connection
            _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
            return err
        }

        pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
        if err != nil {
            initErr = err
            return
        }

        clientInstance = &Client{pool: pool}
    })

    if initErr != nil {
        return nil, initErr
    }

    return clientInstance, nil
}

func (c *Client) SimilaritySearch(ctx context.Context, embedding []float32, topK int) ([]*Result, error) {
    // Use connection from pool
    query := `
        SELECT id, content, metadata, embedding <=> $1 AS distance
        FROM knowledge_vectors
        ORDER BY embedding <=> $1
        LIMIT $2
    `

    rows, err := c.pool.Query(ctx, query, pgvector.NewVector(embedding), topK)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // ... parse results
}

func (c *Client) Close() {
    if c.pool != nil {
        c.pool.Close()
    }
}

// In retriever.go - use singleton
func NewRetriever(config *Config, embedder Embedder) (*Retriever, error) {
    // Don't pass client - will get singleton on first use
    return &Retriever{
        config:   config,
        embedder: embedder,
    }, nil
}

func (r *Retriever) Search(ctx context.Context, query string, topK int) ([]*Result, error) {
    // Get shared client
    client, err := vector.GetClient(ctx, r.config)
    if err != nil {
        return nil, err
    }
    // DON'T close - it's shared

    embedding, err := r.embedder.Embed(ctx, query)
    if err != nil {
        return nil, err
    }

    return client.SimilaritySearch(ctx, embedding, topK)
}
```

**Alternative - Pass Client via Context:**

```go
// engine/knowledge/retrieval/retriever.go
type Retriever struct {
    client   *vector.Client // Store client as field
    embedder Embedder
}

func NewRetriever(client *vector.Client, embedder Embedder) *Retriever {
    return &Retriever{
        client:   client,
        embedder: embedder,
    }
}

func (r *Retriever) Search(ctx context.Context, query string, topK int) ([]*Result, error) {
    embedding, err := r.embedder.Embed(ctx, query)
    if err != nil {
        return nil, err
    }

    // Use injected client
    return r.client.SimilaritySearch(ctx, embedding, topK)
}

// In server initialization:
func setupKnowledgeSystem(ctx context.Context, cfg *config.Config) (*knowledge.System, error) {
    // Create singleton client once
    vectorClient, err := vector.GetClient(ctx, cfg.Vector)
    if err != nil {
        return nil, err
    }

    embedder := embeddings.NewOpenAI(cfg.Embeddings)
    retriever := retrieval.NewRetriever(vectorClient, embedder)

    return knowledge.NewSystem(retriever), nil
}
```

**Impact:**

- Reduces retrieval latency by 10-50ms per request (30-80% improvement)
- Eliminates connection exhaustion under load
- Proper connection reuse and pooling
- Extension check runs once per connection instead of per request

**Testing:**

```bash
# Load test with connection tracking
go test -run TestRetrieverLoadTest -count=100 -parallel=10 ./engine/knowledge/retrieval

# Monitor connection count
SELECT count(*) FROM pg_stat_activity WHERE application_name = 'compozy';

# Should stay at ~20 connections instead of growing unbounded
```

**Effort:** M (3-4h including migration)  
**Risk:** Medium - requires careful singleton pattern

---

### 2. PGVector Similarity Probes Scale with topK

**Location:** `engine/memory/vector/pgvector_client.go:156–203`

**Severity:** 🔴 HIGH

**Issue:**

```go
// Vector similarity search probes increase with topK
func (c *Client) SimilaritySearch(ctx context.Context, embedding []float32, topK int) ([]*Result, error) {
    query := `
        SELECT id, content, metadata, embedding <=> $1 AS distance
        FROM knowledge_vectors
        ORDER BY embedding <=> $1
        LIMIT $2
    `

    // When topK=50, probes ~50 index pages
    // When topK=100, probes ~100 index pages
    rows, err := c.pool.Query(ctx, query, pgvector.NewVector(embedding), topK)
    // ...
}
```

**Problem:** PGVector IVFFlat index probes scale with LIMIT, causing linear latency growth

**Fix - Set Fixed Probe Count:**

```go
// Configure IVFFlat index with fixed probes
func (c *Client) SimilaritySearch(ctx context.Context, embedding []float32, topK int) ([]*Result, error) {
    // Set probes based on index lists, NOT topK
    // If index has 100 lists, probe 10 lists (10%)
    _, err := c.pool.Exec(ctx, "SET ivfflat.probes = 10")
    if err != nil {
        return nil, fmt.Errorf("failed to set probes: %w", err)
    }

    query := `
        SELECT id, content, metadata, embedding <=> $1 AS distance
        FROM knowledge_vectors
        ORDER BY embedding <=> $1
        LIMIT $2
    `

    rows, err := c.pool.Query(ctx, query, pgvector.NewVector(embedding), topK)
    // ...
}

// Create index with optimal list count
func (c *Client) CreateIndex(ctx context.Context) error {
    // For N rows, use lists = sqrt(N)
    // Example: 1M documents → 1000 lists
    query := `
        CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_knowledge_vectors_embedding
        ON knowledge_vectors USING ivfflat (embedding vector_cosine_ops)
        WITH (lists = 1000)
    `

    _, err := c.pool.Exec(ctx, query)
    return err
}
```

**Alternative - Use HNSW Index:**

```sql
-- HNSW doesn't have probe scaling issue
CREATE INDEX CONCURRENTLY idx_knowledge_vectors_embedding_hnsw
ON knowledge_vectors USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- Runtime: set ef_search (independent of topK)
SET hnsw.ef_search = 40;
```

**Configuration:**

```yaml
# config.yaml
knowledge:
  vector:
    index_type: hnsw # or ivfflat
    hnsw:
      m: 16
      ef_construction: 64
      ef_search: 40
    ivfflat:
      lists: 1000
      probes: 10
```

**Impact:**

- Consistent query time regardless of topK
- topK=10: 20ms → 15ms
- topK=100: 150ms → 18ms (8x improvement)

**Benchmarks:**

```go
func BenchmarkSimilaritySearch(b *testing.B) {
    client := setupClient()
    embedding := generateTestEmbedding()

    for _, k := range []int{10, 25, 50, 100} {
        b.Run(fmt.Sprintf("topK=%d", k), func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                _, err := client.SimilaritySearch(ctx, embedding, k)
                if err != nil {
                    b.Fatal(err)
                }
            }
        })
    }
}

// Before: topK=10: 20ms, topK=100: 150ms
// After:  topK=10: 15ms, topK=100: 18ms
```

**Effort:** M (2-3h including index rebuild)  
**Risk:** Medium - requires index migration

---

### 3. Sequential Document Ingestion

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

---

## Medium Priority Issues

### 4. Missing Embedding Cache

**Location:** `engine/memory/embeddings/openai.go:45–89`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// Same query embedded multiple times
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    // No cache - always calls API
    resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
        Model: "text-embedding-3-small",
        Input: []string{text},
    })
    // ...
}
```

**Fix - Add LRU Cache:**

```go
// engine/memory/embeddings/cached_embedder.go (NEW)
package embeddings

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    lru "github.com/hashicorp/golang-lru/v2"
)

type CachedEmbedder struct {
    underlying Embedder
    cache      *lru.Cache[string, []float32]
}

func NewCachedEmbedder(underlying Embedder, cacheSize int) (*CachedEmbedder, error) {
    cache, err := lru.New[string, []float32](cacheSize)
    if err != nil {
        return nil, err
    }

    return &CachedEmbedder{
        underlying: underlying,
        cache:      cache,
    }, nil
}

func (e *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    key := hashText(text)

    // Check cache
    if embedding, ok := e.cache.Get(key); ok {
        return embedding, nil
    }

    // Generate embedding
    embedding, err := e.underlying.Embed(ctx, text)
    if err != nil {
        return nil, err
    }

    // Cache result
    e.cache.Add(key, embedding)

    return embedding, nil
}

func hashText(text string) string {
    h := sha256.Sum256([]byte(text))
    return hex.EncodeToString(h[:])
}
```

**Configuration:**

```yaml
knowledge:
  embeddings:
    cache:
      enabled: true
      size: 10000 # Cache 10k embeddings
```

**Impact:**

- Cache hit: 200ms → 1ms
- Reduces API costs for repeated queries
- Common in test/debug scenarios

**Effort:** S (2h)  
**Risk:** Low

---

## Low Priority Issues

### 5. Chunk Size Optimization

**Location:** `engine/knowledge/chunking/splitter.go:56–112`

**Issue:** Fixed 512-token chunks may not be optimal

**Fix:** Add adaptive chunking based on content type

**Effort:** M (4h)

---

## Implementation Plan

### Phase 1 - Critical (Week 1)

- [ ] Implement PGVector connection pooling (4h)
- [ ] Migrate to singleton client pattern (2h)
- [ ] Add connection pool monitoring

### Phase 2 - Query Optimization (Week 2)

- [ ] Configure fixed probe count (2h)
- [ ] Evaluate HNSW vs IVFFlat (3h)
- [ ] Rebuild indexes with optimal settings

### Phase 3 - Batch Processing (Week 3)

- [ ] Implement batch ingestion pipeline (6h)
- [ ] Add batch embedding support (2h)
- [ ] Add embedding cache (2h)

---

## Testing Requirements

```bash
# Connection pooling
go test -run TestVectorClientPool -count=100 -parallel=20 ./engine/memory/vector

# Query performance
go test -bench=BenchmarkSimilaritySearch -benchmem ./engine/memory/vector

# Batch ingestion
go test -run TestBatchIngestion ./engine/knowledge/ingestion

# Integration
make test
```

---

## Success Metrics

| Metric                      | Before   | Target    | Measurement      |
| --------------------------- | -------- | --------- | ---------------- |
| Retrieval latency (avg)     | 50ms     | 15ms      | Benchmark        |
| Connection count under load | 100+     | 20        | pg_stat_activity |
| topK=100 query time         | 150ms    | 20ms      | Benchmark        |
| Ingestion throughput        | 3 docs/s | 20 docs/s | Load test        |
| Embedding cache hit rate    | 0%       | 40%       | Metrics          |

---

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Platform Team
