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

## Implementation Priorities

### Phase 1: Critical Indexing Performance (Week 1)

1. ✅ Batch metadata reads (#1) - **4h**
2. ✅ Config parsing cache (#2) - **3h**

### Phase 2: Validation & Parallelization (Week 2)

3. ✅ Cache os.Stat results (#3) - **2h**
4. ✅ Parallel indexing (#4) - **4h**

### Phase 3: API Performance (Week 3)

5. ✅ Optimize GetProject (#5) - **2h**

**Total effort:** 15 hours

## Performance Gains Summary

| Optimization      | Scenario       | Before | After | Improvement |
| ----------------- | -------------- | ------ | ----- | ----------- |
| Batch metadata    | 100 resources  | 400ms  | 50ms  | 8x          |
| Config caching    | Repeated load  | 50ms   | 1ms   | 50x         |
| Stat caching      | Validation     | 100ms  | 85ms  | 1.2x        |
| Parallel indexing | 1000 resources | 5s     | 1s    | 5x          |
| GetProject        | API call       | 15ms   | 5ms   | 3x          |

**Total impact:** 8x indexing, 50x repeated loads, 5x large project indexing
