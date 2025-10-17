---
title: "O(N) Reference Scans for Cascaded Deletes"
group: "ENGINE_GROUP_5_DATA_RESOURCES"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_5_DATA_RESOURCES_PERFORMANCE.md"
issue_index: "3"
sequence: "25"
---

## O(N) Reference Scans for Cascaded Deletes

**Location:** `engine/resources/` stores

**Severity:** 🟡 MEDIUM

**Issue:**
Deleting a resource scans ALL resources to find references (O(N) complexity).

**Fix:** Build reverse reference index

**Impact:**

- **Delete complexity:** O(N) → O(1)
- **Delete latency:** 100ms (1000 resources) → 1ms

**Effort:** L (8h)  
**Risk:** Medium
