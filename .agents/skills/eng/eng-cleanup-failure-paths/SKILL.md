---
name: eng-cleanup-failure-paths
description: >-
  Partial-failure cleanup audit for Compozy Go functions. Use when a changed
  function acquires, registers, starts, claims, leases, or opens more than one
  fallible resource before returning. Do not use for pure transformations,
  read-only helpers, or test-only code.
trigger: implicit
---

# Cleanup Failure Paths

Prove ownership across the affected function's acquisitions and exits. Account for the whole lifetime when a local edit changes cleanup; a matrix in working notes is useful for complex ownership, not a required report.

- Identify acquired resources, their owners, cleanup/error policy, shutdown order, and any explicit ownership transfer. Use the matching rows of `references/cleanup-table.md`; read the complete table when the ownership chain spans its rules.
- Walk success, error, cancellation, and reachable recovery exits after each acquisition. Release or transfer every live resource. Preserve the primary error and handle secondary cleanup failures.
- Keep responses and writers request-bound. Detached execution needs a bounded lifetime and cancel/stop surface; subprocess trees retain process-group parity and cancel-then-grace shutdown.
- Repair affected exits together. Split ownership when it cannot be proved locally, not because a function crosses an arbitrary defer-count threshold. Leave unrelated cleanup debt outside the change.

Before changing coverage, name the invariant, owning layer, and existing suite. Use `eng-consolidate-test-suites` only if placement is unclear and `eng-test-conventions` for Go test shape. `references/test-failure-paths.md` supplies patterns for the distinct failure class being changed. Reuse coverage that already proves the invariant; multiple syntactic returns do not imply multiple tests.

Inject failures through an interface or real I/O boundary and assert actual release, reuse, closure, or termination. A mock-call count alone does not prove cleanup. Complete when affected ownership paths are sound and their owning checks pass; the enclosing workstream owns delivery gates.
