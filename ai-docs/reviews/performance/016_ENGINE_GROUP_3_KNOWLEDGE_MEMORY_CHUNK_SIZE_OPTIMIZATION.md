---
title: "Chunk Size Optimization"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "performance"
priority: ""
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_PERFORMANCE.md"
issue_index: "5"
sequence: "16"
---

## Chunk Size Optimization

**Location:** `engine/knowledge/chunking/splitter.go:56–112`

**Issue:** Fixed 512-token chunks may not be optimal

**Fix:** Add adaptive chunking based on content type

**Effort:** M (4h)

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

## Success Metrics

| Metric                      | Before   | Target    | Measurement      |
| --------------------------- | -------- | --------- | ---------------- |
| Retrieval latency (avg)     | 50ms     | 15ms      | Benchmark        |
| Connection count under load | 100+     | 20        | pg_stat_activity |
| topK=100 query time         | 150ms    | 20ms      | Benchmark        |
| Ingestion throughput        | 3 docs/s | 20 docs/s | Load test        |
| Embedding cache hit rate    | 0%       | 40%       | Metrics          |

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Platform Team
