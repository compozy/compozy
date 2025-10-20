# Issue 023 - Review Thread Comment

**File:** `engine/agent/uc/store_list.go:1`
**Date:** 2025-10-20 04:52:28 +0000 UTC
**Status:** - [ ] UNRESOLVED

## Body

### Code Review: store_list.go - Performance

**Review Type:** Performance
**Severity:** Medium

#### Summary

The `List` use‑case loads all agents from the resource store, applies prefix and workflow filters, and then builds a payload for pagination. The implementation is straightforward but contains several patterns that can increase memory allocations, GC pressure, and overall latency, especially for projects with many agents.

#### Findings

### 🔴 Critical Issues

_None identified._

### 🟠 High Priority Issues

- **Inefficient full store scan**
  - **Problem**: `uc.store.ListWithValues(ctx, projectID, resources.ResourceAgent)` retrieves **all** agents for a project, regardless of the requested `Prefix` or `WorkflowID`. For large projects this results in a massive slice allocation and unnecessary decoding work.
  - **Impact**: High memory usage, increased GC pressure, and slower response times.
  - **Fix**: Add a store method that can filter by prefix and/or workflow ID at the storage layer (e.g., `ListAgents(ctx, projectID, prefix, workflowIDs)`). If the underlying store cannot filter, consider streaming results instead of loading the entire slice.
  - **Rule Reference**: `.cursor/rules/go-coding-standards.mdc` – _Database query optimization_; _Avoid loading unnecessary data_.

  ```go
  // ❌ Current implementation
  items, err := uc.store.ListWithValues(ctx, projectID, resources.ResourceAgent)

  // ✅ Recommended fix (pseudo‑code)
  // Let the store apply the prefix and workflow filters directly.
  items, err := uc.store.ListAgents(ctx, projectID, strings.TrimSpace(in.Prefix), filterIDs)
  ```

- **Repeated map allocation for workflow filter**
  - **Problem**: `workflowFilter` always creates `filters := map[string]struct{}{}` even when `workflowID` is empty, resulting in an unnecessary allocation on every request.
  - **Impact**: Minor but adds up under high request volume.
  - **Fix**: Return a `nil` map when no workflow filter is needed and adjust callers to treat a nil map as “no filter”.
  - **Rule Reference**: `.cursor/rules/go-coding-standards.mdc` – _Memory allocations_.

  ```go
  // ❌ Current implementation
  filters := map[string]struct{}{}
  if id == "" {
      return filters, nil
  }

  // ✅ Recommended fix
  if id == "" {
      return nil, nil // nil signals no filter
  }
  filters := make(map[string]struct{}, expectedSize)
  ```

### 🟡 Medium Priority Issues

- **Unnecessary slice capacity over‑allocation**
  - **Problem**: `payload := make([]map[string]any, 0, len(window))` is fine, but `ids := make([]string, 0, len(set))` creates a slice that may be re‑allocated when the set size is small. This is negligible but can be tuned.
  - **Fix**: Use `make([]string, 0, len(set))` only when `len(set) > 0`; otherwise return an empty slice directly.

- **Repeated string trimming**
  - **Problem**: `strings.TrimSpace` is called multiple times on the same fields (`in.Prefix`, `in.CursorValue`, `workflowID`).
  - **Fix**: Trim once and reuse the trimmed value.

- **Missing context‑based logging**
  - **Problem**: Errors are returned without logging context information, making troubleshooting harder.
  - **Fix**: Use `logger := logger.FromContext(ctx)` and log errors before returning.

  ```go
  // ❌ Current error return
  if err != nil {
      return nil, err
  }

  // ✅ Recommended pattern
  logger := logger.FromContext(ctx)
  if err != nil {
      logger.Error("failed to list agents", "err", err)
      return nil, err
  }
  ```

### 🔵 Low Priority / Suggestions

- **Cache workflow‑agent mappings**
  - **Suggestion**: Cache the result of `workflowAgents` per `(projectID, workflowID)` for the lifetime of a request or a short TTL. This avoids decoding the same workflow repeatedly when multiple list calls are made in quick succession.
  - **Benefit**: Reduces CPU work and memory churn.

- **Use core.CopyMap / core.CloneMap for map copies**
  - **Suggestion**: If a copy of the `allow` map is ever needed, prefer `core.CopyMap` to keep consistency with project conventions.

#### Rule References

- `.cursor/rules/go-coding-standards.mdc`: Sections on _Memory allocations_, _Database query optimization_, _Map operations_, _Context propagation_.
- `.cursor/rules/architecture.mdc`: Emphasizes _Dependency injection_ and _Clean Architecture_ – the store should expose filtered query methods.

#### Impact Assessment

- **Performance Impact**: Reducing full store scans can cut latency by >50% for large projects and dramatically lower GC pressure.
- **Maintainability Impact**: Introducing filtered store methods centralizes query logic, making future changes easier.
- **Security Impact**: None directly, but better logging aids incident investigation.
- **Reliability Impact**: Lower memory usage reduces out‑of‑memory crashes under load.

#### Recommendations

**Immediate Actions (High Priority)**

1. Implement a filtered `ListAgents` method in `resources.ResourceStore` and replace the full `ListWithValues` call.
2. Refactor `workflowFilter` to return `nil` when no workflow ID is supplied and adjust callers accordingly.

**Short‑term Improvements (Medium Priority)**

1. Consolidate `strings.TrimSpace` calls.
2. Add context‑based logging for error paths.
3. Review slice capacity allocations for small collections.

**Long‑term Enhancements (Low Priority)**

1. Introduce a short‑lived cache for workflow‑agent mappings.
2. Align any map copying with `core.CopyMap`/`core.CloneMap` utilities.

#### Positive Aspects

- The code follows clean‑architecture boundaries and uses dependency injection for the store.
- Pagination logic is delegated to `resourceutil.ApplyCursorWindow`, keeping the use‑case focused.
- Errors are wrapped with contextual messages, adhering to error‑handling standards.

## Resolve

_Note: This issue was generated from code review analysis._

**Original analysis type:** performance
**File analyzed:** `engine/agent/uc/store_list.go`

To mark this issue as resolved:

1. Update this file's status line by changing `[ ]` to `[x]`
2. Update the grouped summary file
3. Update `_summary.md`

---

_Generated from code review analysis_
