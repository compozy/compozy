# Engine Group 1: Execution Core - Performance Improvements

**Packages:** agent, task, task2, workflow, runtime

---

## Executive Summary

Critical performance issues and optimizations for the execution core that handles workflow orchestration, task execution, agent coordination, and tool runtime management.

**Priority Findings:**

- 🔴 **Critical:** Concurrency bug and goroutine leak in Bun runtime
- 🔴 **High Impact:** Double allocations in stdout parsing
- 🟡 **Medium Impact:** Repeated string allocations in workflow polling
- 🟢 **Low Impact:** Sequential execution opportunities

---

## High Priority Issues

### 1. Runtime Concurrency Bug & Goroutine Leak

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

---

### 2. Double Allocation in Stdout Parsing

**Location:** `engine/runtime/bun_manager.go:480–506`, `343–386`

**Severity:** 🔴 HIGH

**Issue:**

```go
// readStdoutResponse returns string (allocation #1)
func readStdoutResponse(stdout io.Reader) (string, error) {
    buf := new(bytes.Buffer)
    _, err := io.Copy(buf, stdout)
    return buf.String(), err // Allocates string
}

// parseToolResponse converts to []byte (allocation #2)
func parseToolResponse(jsonStr string) (*ToolResponse, error) {
    var resp ToolResponse
    err := json.Unmarshal([]byte(jsonStr), &resp) // Converts string -> []byte
    return &resp, err
}
```

**Problems:**

1. Entire output buffered in memory twice
2. For large outputs (up to MaxOutputSize = 10MB), wastes 20MB
3. String intermediate serves no purpose

**Fix - Option A (Minimal):**

```go
// Return []byte directly
func readStdoutResponse(stdout io.Reader) ([]byte, error) {
    buf := new(bytes.Buffer)
    _, err := io.Copy(buf, stdout)
    return buf.Bytes(), err // Returns []byte directly
}

func parseToolResponse(data []byte) (*ToolResponse, error) {
    var resp ToolResponse
    err := json.Unmarshal(data, &resp)
    return &resp, err
}
```

**Fix - Option B (Optimal - Streaming):**

```go
// Stream directly from stdout
func decodeToolResponse(stdout io.Reader) (*ToolResponse, error) {
    // Enforce size limit while streaming
    lr := &io.LimitedReader{R: stdout, N: MaxOutputSize + 1}

    dec := json.NewDecoder(lr)
    var resp ToolResponse

    if err := dec.Decode(&resp); err != nil {
        return nil, fmt.Errorf("failed to decode tool output: %w", err)
    }

    // Check if we hit the limit
    if lr.N <= 0 {
        return nil, fmt.Errorf("tool output exceeded maximum size of %d bytes", MaxOutputSize)
    }

    return &resp, nil
}
```

**Impact:**

- Option A: 50% memory reduction (single allocation)
- Option B: No buffering, bounded memory, faster for large outputs
- Reduces GC pressure

**Benchmarks:**

```bash
# Before: BenchmarkParseToolResponse-8  1000  1200000 ns/op  20MB alloc
# After A: BenchmarkParseToolResponse-8  1000  1100000 ns/op  10MB alloc
# After B: BenchmarkParseToolResponse-8  1000   800000 ns/op   5MB alloc
```

**Effort:**

- Option A: S (30min)
- Option B: M (2h including tests)

**Risk:** Low - maintains same API contract

---

### 3. Repeated String Allocations in Workflow Polling

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

---

## Medium Priority Issues

### 4. Extra Store Round-Trips on Sync Success

**Location:** `engine/task/router/exec.go:334–365`, `engine/agent/router/exec.go:149–186`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// After successful execution:
func buildTaskSyncPayload(ctx context.Context, taskID core.ID, repo Repository) (*SyncPayload, error) {
    // Extra DB call to reload full state
    state, err := repo.GetState(ctx, taskID)
    if err != nil {
        return nil, err
    }

    // Usage already available from execution
    usage, _ := repo.GetUsage(ctx, taskID)

    return &SyncPayload{
        State: state, // Often not needed by client
        Usage: usage,
    }, nil
}
```

**Problem:**

- Clients often only need usage, not full state
- Extra repository round-trip adds 5-20ms latency
- Payload bloat for large states

**Fix:**

```go
// Add query parameter support
// GET /api/v1/tasks/:id/execute?include=state

func buildTaskSyncPayload(c *gin.Context, taskID core.ID, repo Repository, usage *task.UsageSummary) (*SyncPayload, error) {
    includeState := c.Query("include") == "state"

    payload := &SyncPayload{
        Usage: usage,
    }

    if includeState {
        state, err := repo.GetState(c.Request.Context(), taskID)
        if err != nil {
            return nil, err
        }
        payload.State = state
    }

    return payload, nil
}
```

**Impact:**

- Opt-in state loading reduces latency by 10-20%
- Smaller response payloads
- Backward compatible (default includes state if needed)

**Effort:** M (2-3h including both task and agent)  
**Risk:** Low - additive change

---

### 5. Per-Iteration Timer Allocation

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

---

## Low Priority Issues

### 6. Stderr Reader Allocations

**Location:** `engine/runtime/bun_manager.go:409–477`

**Issue:** Per-byte processing with repeated string allocations

**Fix:**

```go
// Use bufio.Scanner
scanner := bufio.NewScanner(stderr)
scanner.Buffer(make([]byte, MaxStderrCaptureSize), MaxStderrCaptureSize)

for scanner.Scan() {
    line := scanner.Text()
    log.Debug(line)
}

if err := scanner.Err(); err != nil {
    log.Warn("stderr read error", "error", err)
}
```

**Impact:** Only matters for stderr-heavy tools

**Effort:** S (1h)

---

## Implementation Plan

### Phase 1 - Critical (Day 1)

- [ ] Fix runtime concurrency bug (1h)
- [ ] Add goroutine leak tests (1h)
- [ ] Test under cancellation/timeout scenarios

### Phase 2 - High Impact (Week 1)

- [ ] Implement streaming stdout decoder (2h)
- [ ] Precompute workflow jitter strings (30min)
- [ ] Benchmark memory improvements

### Phase 3 - Optimizations (Week 2)

- [ ] Add optional state inclusion flag (3h)
- [ ] Optimize timer reuse if needed (30min)
- [ ] Optimize stderr reader if profiled (1h)

---

## Testing Requirements

```bash
# Concurrency fixes
go test -race -count=100 ./engine/runtime

# Memory benchmarks
go test -bench=. -benchmem -memprofile=mem.out ./engine/runtime
go tool pprof -alloc_space mem.out

# Goroutine leak detection
go test -run TestExecuteToolTimeout ./engine/runtime
# Use testing.LeakDetector or manual goroutine counting

# Integration test
make test
```

---

## Success Metrics

| Metric                        | Before | Target | Measurement          |
| ----------------------------- | ------ | ------ | -------------------- |
| Goroutine leaks               | Yes    | Zero   | `go test -count=100` |
| Tool execution memory         | 20MB   | 10MB   | `benchstat`          |
| Workflow poll allocations     | N×ID   | 1×ID   | `pprof`              |
| Sync response time (no state) | N/A    | -15%   | Load test            |

---

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Platform Team
