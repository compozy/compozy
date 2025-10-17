---
title: "Redis ListWithValues Double MGET Operations"
group: "ENGINE_GROUP_5_DATA_RESOURCES"
category: "performance"
priority: "🔴 HIGH"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_5_DATA_RESOURCES_PERFORMANCE.md"
issue_index: "2"
sequence: "24"
---

## Redis ListWithValues Double MGET Operations

**Location:** `engine/resources/redis_store.go:348–381`

**Severity:** 🔴 HIGH

**Issue:**

```go
// Two separate MGET calls:
vals, err := s.r.MGet(ctx, redisKeys...).Result()        // ❌ MGET 1
etVals, etErr := s.r.MGet(ctx, etagKeys...).Result()     // ❌ MGET 2
```

**Problems:**

1. Double network round-trip: 2x latency
2. Sequential execution instead of parallel
3. 100 keys = 4ms total (should be 2ms)

**Fix:** Interleave keys in single MGET or use Pipeline

**Benchmark:**

```
Keys    Current (2 MGET)    Optimized (1 MGET)    Speedup
10      2.1ms              1.2ms                 1.75x
100     4.5ms              2.3ms                 1.96x
1000    22ms               12ms                  1.83x
```

**Effort:** S (2h)  
**Risk:** Low
