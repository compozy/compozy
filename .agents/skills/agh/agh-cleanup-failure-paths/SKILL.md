---
name: agh-cleanup-failure-paths
description: >-
  Partial-failure cleanup audit for AGH Go functions. Use when a changed
  function acquires, registers, starts, claims, leases, or opens more than one
  fallible resource before returning. Do not use for pure transformations,
  read-only helpers, or test-only code.
trigger: implicit
---

# Cleanup Failure Paths

Audit the complete acquisition/cleanup matrix of every touched function. This
file owns the audit order; its references own canonical pairings and tests.

## Procedures

**Step 1: Build the Acquisition Matrix**

1. Read the target function and enumerate every "step" that allocates a resource: context creation, file open, listener bind, registry register, lease claim, goroutine spawn, subprocess start, HTTP request, mutex lock, transaction begin.
2. Read `.agents/skills/agh/agh-cleanup-failure-paths/references/cleanup-table.md` in full and map each acquisition to its owner, cleanup action, cleanup-error handling, and shutdown order.
3. Mark resources whose lifetime intentionally escapes the function and name the new owner and explicit cancel/stop surface.

*Done when:* every acquired resource has one owner, one cleanup path, one error policy, and one bounded lifetime.

**Step 2: Walk Every Exit**

1. List success, error, cancellation, panic-recovery, and `runtime.Goexit` exits after each acquisition.
2. Walk backward from every exit and account for all resources acquired above it.
3. Handle cleanup failures according to the canonical reference; do not replace a primary error or silently discard a secondary cleanup error.
4. Restructure the function when ownership cannot be proven locally instead of auditing only the branch being edited.

*Done when:* every exit after every acquisition releases or transfers ownership of every live resource, including cleanup-error paths.

**Step 3: Apply Lifetime and Shutdown Semantics**

1. Apply every matching pairing, ordering, detached-lifetime, subprocess, transaction, and HTTP-drain rule from the cleanup table.
2. Keep writer/response lifetimes request-bound while explicitly detached execution gets a deadline and cancel surface.
3. Verify process-group parity and cancel-then-grace semantics for subprocess trees.

*Done when:* cleanup order preserves public/private state invariants and every detached or spawned lifetime has an observable stop path.

**Step 4: Prove Distinct Failure Modes**

1. Activate `consolidate-test-suites` and `agh-test-conventions` before changing Go tests.
2. Read `.agents/skills/agh/agh-cleanup-failure-paths/references/test-failure-paths.md` in full.
3. Add or extend canonical coverage for each behaviorally distinct cleanup invariant, not each syntactic return.
4. Inject the failure through an interface or real boundary, then assert the resource is actually reusable, released, stopped, or closed; a mock-call count alone is insufficient.

*Done when:* every distinct failure class that could leak ownership is proved by the canonical suite without duplicate invariants.

## Error Handling

- **Function is too large to audit completely:** split ownership into smaller helpers before changing behavior; completion still requires every exit in the touched function to be accounted for.
- **Cleanup requires a resource not in scope:** the function is structured wrong — push the cleanup responsibility up to the caller via an opener-closer pair, or restructure into a helper that returns a `cleanup func()` callback.
- **Existing function lacks error-path cleanup:** treat the touched function as one cleanup unit and repair every affected exit before completion.
- **`defer` count exceeds reasonable bounds (e.g., >6 in one function):** the function is doing too much. Recommend splitting before adding more defers.
