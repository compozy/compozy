---
title: "Sequential Resource Indexing"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_PERFORMANCE.md"
issue_index: "4"
sequence: "31"
---

## Sequential Resource Indexing

**Location:** `engine/project/indexer.go:17-48`

**Severity:** 🟡 MEDIUM

**Issue:**
Resources indexed sequentially instead of parallel.

**Fix:** Use worker pool for parallel indexing

**Impact:** 3-5x faster indexing for large projects

**Effort:** M (4h)  
**Risk:** Low
