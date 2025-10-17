---
title: "Memory Store Broadcast Under Write Lock"
group: "ENGINE_GROUP_5_DATA_RESOURCES"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_5_DATA_RESOURCES_PERFORMANCE.md"
issue_index: "4"
sequence: "26"
---

## Memory Store Broadcast Under Write Lock

**Location:** `engine/resources/memory_store.go`

**Severity:** 🟡 MEDIUM

**Issue:**
Store broadcasts to watchers while holding write lock, blocking other operations.

**Fix:** Release lock before broadcasting, use async notifications

**Impact:**

- **Write throughput:** 2-10x improvement
- **P99 latency:** 50ms → 5ms

**Effort:** M (3h)  
**Risk:** Low
