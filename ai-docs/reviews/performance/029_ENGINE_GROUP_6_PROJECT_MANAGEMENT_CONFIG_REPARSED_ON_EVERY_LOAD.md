---
title: "Config Reparsed on Every Load"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "performance"
priority: "🔴 HIGH"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_PERFORMANCE.md"
issue_index: "2"
sequence: "29"
---

## Config Reparsed on Every Load

**Location:** `engine/project/config.go`

**Severity:** 🔴 HIGH

**Issue:**
Project config file is parsed on every call to `Load()` even if file hasn't changed.

**Fix:** Add file modification time checking and caching

**Impact:**

- **Load time:** 50ms → 1ms for unchanged config
- **CPU:** 95% reduction for repeated loads

**Effort:** M (3h)  
**Risk:** Low
