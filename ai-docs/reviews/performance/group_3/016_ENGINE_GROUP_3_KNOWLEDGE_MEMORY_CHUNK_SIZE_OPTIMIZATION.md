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
