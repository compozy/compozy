---
title: "GetProject Uses Reflection-Heavy AsMap"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "performance"
priority: "🟢 LOW"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_PERFORMANCE.md"
issue_index: "5"
sequence: "32"
---

## GetProject Uses Reflection-Heavy AsMap

**Location:** `engine/project/` (GetProject method)

**Severity:** 🟢 LOW

**Issue:**
Converting project config to map using reflection is slow.

**Fix:** Use direct struct field access or JSON marshal

**Impact:** 2-3x faster for API responses

**Effort:** S (2h)  
**Risk:** None
