---
title: "Per-Iteration Timer Allocation"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "performance"
priority: "🟢 LOW"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_PERFORMANCE.md"
issue_index: "5"
sequence: "5"
---

## Per-Iteration Timer Allocation

**Location:** `engine/workflow/router/execute_sync.go:275–289`

**Severity:** 🟢 LOW

**Issue:**

```go
// In polling loop:
for {
    timer := time.NewTimer(backoff) // New timer allocation
    select {
    case <-timer.C:
        // ...
    case <-ctx.Done():
        timer.Stop()
        return
    }
}
```

**Fix (Optional):**

```go
// Reuse single timer
timer := time.NewTimer(backoff)
defer timer.Stop()

for {
    select {
    case <-timer.C:
        // ... process
        timer.Reset(nextBackoff) // Reuse
    case <-ctx.Done():
        return
    }
}
```

**Impact:** Marginal - only optimize if profiler shows significance

**Effort:** S (30min)  
**Risk:** None

## Low Priority Issues
