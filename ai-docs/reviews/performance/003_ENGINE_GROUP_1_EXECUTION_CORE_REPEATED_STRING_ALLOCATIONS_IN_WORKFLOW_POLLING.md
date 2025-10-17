---
title: "Repeated String Allocations in Workflow Polling"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_PERFORMANCE.md"
issue_index: "3"
sequence: "3"
---

## Repeated String Allocations in Workflow Polling

**Location:** `engine/workflow/router/execute_sync.go:220–254`, `319–344`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// Called on every poll iteration (potentially 100+ times)
func applyWorkflowJitter(base time.Duration, execID core.ID, attempt int) time.Duration {
    // execID.String() allocates every call
    seed := hashString(execID.String() + strconv.Itoa(attempt))
    // ...
}

// In polling loop:
for {
    // ...
    backoff = applyWorkflowJitter(backoff, execID, attempt) // Allocates string
    attempt++
}
```

**Problem:** For 100 poll iterations, allocates 100 strings of same ID

**Fix:**

```go
// Precompute once before loop
execIDStr := execID.String()

// Modified function signature
func applyWorkflowJitterStr(base time.Duration, execIDStr string, attempt int) time.Duration {
    seed := hashString(execIDStr + strconv.Itoa(attempt))
    // ... rest unchanged
}

// In polling loop:
for {
    // ...
    backoff = applyWorkflowJitterStr(backoff, execIDStr, attempt)
    attempt++
}
```

**Impact:**

- Eliminates N allocations per workflow sync execution
- Reduces CPU for string conversion
- Marginal but free optimization

**Effort:** S (30min)  
**Risk:** None

## Medium Priority Issues
