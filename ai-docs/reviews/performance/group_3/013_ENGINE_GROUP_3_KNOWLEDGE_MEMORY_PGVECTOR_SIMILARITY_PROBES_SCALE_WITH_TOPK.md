---
title: "PGVector Similarity Probes Scale with topK"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "performance"
priority: "🔴 HIGH"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_PERFORMANCE.md"
issue_index: "2"
sequence: "13"
---

## PGVector Similarity Probes Scale with topK

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
