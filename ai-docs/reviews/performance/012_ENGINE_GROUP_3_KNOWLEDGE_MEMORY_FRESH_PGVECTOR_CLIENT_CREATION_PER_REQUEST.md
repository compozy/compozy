---
title: "Fresh PGVector Client Creation Per Request"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "performance"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_PERFORMANCE.md"
issue_index: "1"
sequence: "12"
---

## Fresh PGVector Client Creation Per Request

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
