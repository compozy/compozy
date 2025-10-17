---
title: "Runtime Concurrency Bug & Goroutine Leak"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "performance"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_PERFORMANCE.md"
issue_index: "1"
sequence: "1"
---

## Runtime Concurrency Bug & Goroutine Leak

**Location:** `engine/runtime/bun_manager.go:417–477`, `508–574`

**Severity:** 🔴 CRITICAL

**Issue:**

```go
// Line 417 - WRONG: WaitGroup has no .Go() method
stderrWg.Go(func() { ... })

// Line 432 - WRONG: Cannot range over int
for i := range n {

// Lines 515-519 - WRONG: Early return without cleanup
if ctx.Err() != nil {
    return fmt.Errorf("bun process canceled: %w", ctx.Err())
}
```

**Problems:**

1. `sync.WaitGroup.Go()` doesn't exist - code won't compile or will panic
2. Early return on cancellation leaves stderr goroutine running
3. Invalid loop construct causes runtime error
4. File descriptors and goroutines leak under timeout/cancellation

**Fix:**

```go
// Line 417 - CORRECT:
stderrWg.Add(1)
go func() {
    defer stderrWg.Done()
    // ... existing stderr reading logic
}()

// Line 432 - CORRECT:
for i := 0; i < n; i++ {
    // ... existing logic
}

// Lines 515-519 - CORRECT: Always wait for cleanup
// Remove early return completely
waitErr := cmd.Wait()
stderrWg.Wait() // Always wait for stderr goroutine

// Then map errors appropriately:
if ctx.Err() == context.DeadlineExceeded {
    return &TimeoutError{...}
} else if ctx.Err() != nil {
    return &CanceledError{...}
} else if waitErr != nil {
    return &ProcessError{...}
}
return nil
```

**Impact:**

- Prevents goroutine leaks (can accumulate to thousands under load)
- Prevents file descriptor exhaustion
- Ensures stderr is always flushed for debugging

**Testing:**

```bash
# Test for goroutine leaks
go test -run TestExecuteToolTimeout -count=100 ./engine/runtime
# Check goroutine count doesn't grow

# Test cancellation
go test -run TestExecuteToolCancel ./engine/runtime
```

**Effort:** S (1h)  
**Risk:** None - fixes broken code
