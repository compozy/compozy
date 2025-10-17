---
title: "Missing Embedding Cache"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_PERFORMANCE.md"
issue_index: "4"
sequence: "15"
---

## Missing Embedding Cache

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

## Low Priority Issues
