---
title: "Stderr Reader Allocations"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "performance"
priority: ""
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_PERFORMANCE.md"
issue_index: "6"
sequence: "6"
---

## Stderr Reader Allocations

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

## Success Metrics

| Metric                        | Before | Target | Measurement          |
| ----------------------------- | ------ | ------ | -------------------- |
| Goroutine leaks               | Yes    | Zero   | `go test -count=100` |
| Tool execution memory         | 20MB   | 10MB   | `benchstat`          |
| Workflow poll allocations     | N×ID   | 1×ID   | `pprof`              |
| Sync response time (no state) | N/A    | -15%   | Load test            |

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Platform Team
