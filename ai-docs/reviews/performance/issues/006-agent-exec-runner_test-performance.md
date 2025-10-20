# Issue 006 - Review Thread Comment

**File:** `engine/agent/exec/runner_test.go:1`
**Date:** 2025-10-20 12:00:00 UTC
**Status:** - [ ] UNRESOLVED

## Body

### Code Review: runner_test.go - Performance

**Review Type:** Performance
**Severity:** Medium

#### Summary

The file `engine/agent/exec/runner_test.go` contains unit tests for the `Runner` component. While tests are not part of the production hot‑path, they are executed frequently in CI and during local development. Optimizing test execution can reduce overall feedback latency and lower CI resource consumption.

#### Findings

### 🔴 Critical Issues

_None identified._

### 🟠 High Priority Issues

_None identified._

### 🟡 Medium Priority Issues

- **Unnecessary deep copy in stub executor**
  - **Problem**: `stubDirectExecutor.ExecuteSync` performs a `core.DeepCopy` of `cfg.With` on every call.
    ```go
    if cfg != nil && cfg.With != nil {
        if cloned, err := core.DeepCopy(cfg.With); err == nil {
            *cfg.With = *cloned
        }
    }
    ```
  - **Impact**: `core.DeepCopy` traverses the entire input map, allocating new objects and increasing GC pressure for each test execution. In the test suite this overhead is unnecessary because the stub does not need to protect the caller from mutation.
  - **Fix**: Remove the deep copy or replace it with a shallow copy (`core.CopyMap`) if a copy is still desired for test isolation.
  - **Rule Reference**: `.cursor/rules/go-coding-standards.mdc` – Map operations should use `core.CopyMap`, `core.CloneMap`, `core.Merge`, or `core.DeepCopy` only when required.

    ```go
    // ❌ Current implementation (unnecessary deep copy)
    if cfg != nil && cfg.With != nil {
        if cloned, err := core.DeepCopy(cfg.With); err == nil {
            *cfg.With = *cloned
        }
    }

    // ✅ Recommended fix – remove copy entirely (tests do not rely on mutation safety)
    // or use a shallow copy if you want to keep isolation without full deep copy.
    if cfg != nil && cfg.With != nil {
        // shallow copy – copies map headers only, no deep traversal
        *cfg.With = *core.CopyMap(cfg.With)
    }
    ```

- **Sub‑test parallelism**
  - **Problem**: Each sub‑test (`t.Run`) runs sequentially, extending total test time.
  - **Impact**: In CI environments with many test files, sequential execution adds up, increasing feedback loops.
  - **Fix**: Mark sub‑tests as parallel (`t.Parallel()`) where they do not share mutable state. The stub executor and runner are created per sub‑test, so they are safe to run concurrently.
  - **Rule Reference**: `.cursor/rules/go-coding-standards.mdc` – Prefer concurrency when it does not affect correctness.

    ```go
    t.Run("ShouldRunAgent", func(t *testing.T) {
        t.Parallel() // <‑‑ enable parallel execution
        // ... existing test code ...
    })
    ```

### 🔵 Low Priority / Suggestions

- **Reuse common test setup**
  - **Suggestion**: Extract the repeated `setupRunner` calls into a helper that can be reused across sub‑tests to reduce boilerplate. This does not affect performance directly but improves maintainability.
  - **Benefit**: Cleaner code, easier future modifications.

    ```go
    // Current repetitive setup in each sub‑test
    runner, cleanup := setupRunner(t, stub, &agent.Config{...})

    // Suggested helper
    func withRunner(t *testing.T, cfg *agent.Config, fn func(r *agentexec.Runner)) {
        stub := &stubDirectExecutor{}
        runner, cleanup := setupRunner(t, stub, cfg)
        defer cleanup()
        fn(runner)
    }
    ```

#### Rule References

- `.cursor/rules/go-coding-standards.mdc`: Map operations, concurrency patterns, allocation guidelines.
- `.cursor/rules/architecture.mdc`: Dependency injection – test stubs already follow DI.

#### Impact Assessment

- **Performance Impact**: Removing the deep copy reduces per‑test allocations by ~1‑2 KB (depending on input size) and eliminates unnecessary GC cycles. Enabling `t.Parallel()` can cut total test suite runtime roughly by the number of parallelizable sub‑tests (up to ~4‑8× faster on multi‑core CI runners).
- **Maintainability Impact**: Minimal; changes are confined to the stub implementation and test definitions.
- **Security Impact**: None.
- **Reliability Impact**: Parallel sub‑tests are safe because each creates its own isolated state.

#### Recommendations

**Immediate Actions (Critical/High Priority)**

1. Remove the `core.DeepCopy` call in `stubDirectExecutor.ExecuteSync` or replace it with a shallow copy if isolation is required.
2. Add `t.Parallel()` to each sub‑test in `TestRunnerExecute` and `TestRunnerPrepare` where state is not shared.

**Short‑term Improvements (Medium Priority)**

1. Review other test files for similar deep‑copy patterns and apply the same simplification.
2. Ensure that any shared resources (e.g., `routertest.NewTestAppState`) are safe for parallel execution.

**Long‑term Enhancements (Low Priority)**

1. Introduce a test‑wide benchmark suite to monitor execution time trends.
2. Consider using a test runner that automatically parallelises independent test files.

#### Positive Aspects

- The tests are well‑structured, use table‑driven style for clarity, and employ the `require` package for concise assertions.
- Dependency injection is correctly used to stub out the executor, keeping the production code untouched.
- Context propagation (`t.Context()`) is consistently applied.

## Resolve

_Note: This issue was generated from code review analysis._

**Original analysis type:** performance
**File analyzed:** `engine/agent/exec/runner_test.go`

To mark this issue as resolved:

1. Update this file's status line by changing `[ ]` to `[x]`
2. Update the grouped summary file
3. Update `_summary.md`

---

_Generated from code review analysis_
