---
provider: manual
pr:
round: 1
round_created_at: 2026-07-23T18:09:25Z
status: resolved
file: internal/core/taskgroups/completion.go
line: 254
severity: medium
author: claude-code
provider_ref:
---

# Issue 004: Bulk completion hydration aborts entirely on one missing heading

## Review Comment

`hydrateCompletionLocked` (`internal/core/taskgroups/completion.go:250-262`)
iterates the authoritative completed task-group IDs and calls
`RewriteCompletion(rewritten, taskGroupID)` for each, returning `nil, rewriteErr`
on the first error. `RewriteCompletion` (`:98-115`) returns `ErrCompletionConflict`
whenever the selected heading matches a number of times other than one — which
includes the **zero-match** case where a completed group's `## [ ] TG-NNN — …`
heading is absent from the plan (e.g. the plan was regenerated or edited to drop
that group). The result: one drifted/renamed heading aborts the whole projection
and **none** of the other, valid completed groups get marked.

This contradicts the documented contract that hydration is an "additive,
idempotent, best-effort projection." Because the daemon calls hydration
best-effort (`hydrateTaskGroupPlanBestEffort` logs a warning and continues),
the failure is silent: a dependent group's launch preflight then reads a stale
`_task_groups.md`, finds its prerequisites unmarked, and is rejected
`task_group_dependencies_unmet` even though those prerequisites are complete in
globaldb (the IT-017 convergence scenario). The user's recovery tool
`compozy tasks sync` funnels through the same `HydrateCompletion` and returns
exit 1 for that initiative, so it dead-ends too.

The completed IDs come straight from globaldb (`completedTaskGroupIDsForInitiative`)
with no cross-check that each has a heading in the current plan file, so the
abort is reachable in practice under plan drift.

Suggested fix: make the bulk hydration path resilient to an absent heading —
skip a completed ID whose heading is not present (mark the rest) rather than
aborting, while still surfacing the genuinely ambiguous (>1 match) case. This
likely means distinguishing the zero-match from the multi-match case (e.g. a
dedicated "heading not found → skip" signal from `RewriteCompletion` or a
pre-filter to headings present in the file). Add a hydration test covering
"globaldb reports a completed group with no heading in the plan → other groups
still marked, no error."

## Triage

- Decision: `VALID`
- Notes:
  - Root cause confirmed by reading the code. `RewriteCompletion`
    (`completion.go`) guards on `len(selected) != 1` and returns the same
    `ErrCompletionConflict` for **both** the zero-match (heading absent) and the
    multi-match (ambiguous) cases, discarding the count that would let a caller
    tell them apart. `hydrateCompletionLocked` then returns `nil, rewriteErr` on
    the first error, so one completed group whose `## [ ] TG-NNN — …` heading is
    missing under plan drift aborts the entire projection and none of the other
    valid groups get marked.
  - The completed IDs come straight from globaldb via
    `CompletedTaskGroupIDsWithDB` with no cross-check against the current plan
    headings, so the abort is reachable in practice (IT-017 convergence
    scenario). Both callers route through this one core: the daemon's
    `hydrateTaskGroupPlanBestEffort` swallows the error with a warning (silent
    stale plan → `task_group_dependencies_unmet`), and `compozy tasks sync` via
    `HydratePlanCompletionWithDB` wraps it into an exit-1 dead-end. This
    contradicts the "additive, idempotent, best-effort projection" contract.
  - Fix (all in `completion.go`): added an internal `rewriteCompletion` that
    returns the heading match count; the public `RewriteCompletion` is an
    unchanged thin wrapper so `MarkComplete` and existing tests keep their exact
    contract. `hydrateCompletionLocked` now skips a completed ID with **zero**
    matching headings (`continue`) and keeps marking the rest, while a genuinely
    ambiguous (>1) heading still aborts fail-closed with `ErrCompletionConflict`
    and writes nothing. Updated the `HydrateCompletion` doc comment to state the
    skip-absent invariant.
  - Tests (`taskgroups_test.go`, `TestHydrateCompletion`): (1) globaldb reports a
    completed group with no heading in the plan → the two present groups are still
    marked and no error is returned; (2) a duplicated heading is surfaced as
    `ErrCompletionConflict` without a partial write (plan bytes unchanged).
  - Verification: `make verify` passes through `fmt`, `lint` (0 issues), `test`
    (5430 passed / 7 skipped, `-race`), `go-build`, and `verify-extensions`. The
    final `frontend-e2e` stage fails at daemon readiness
    (`read daemon info … daemon.json: no such file or directory`, daemon
    subprocess exit 2). This is a pre-existing, environmental failure of the
    sandboxed review worktree — it reproduces identically with a fresh unrelated
    `HOME` and no relation to this change, which touches only the leaf
    `taskgroups` package (no `internal/daemon`, daemon-boot, or frontend code).
    Documented per the cy-fix-reviews unrelated-failure rule; the Go verification
    scope that covers this change is fully green.
