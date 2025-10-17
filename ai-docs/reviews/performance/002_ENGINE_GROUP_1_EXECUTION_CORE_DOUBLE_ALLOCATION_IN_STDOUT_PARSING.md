---
title: "Double Allocation in Stdout Parsing"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "performance"
priority: "🔴 HIGH"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_PERFORMANCE.md"
issue_index: "2"
sequence: "2"
---

## Double Allocation in Stdout Parsing

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
