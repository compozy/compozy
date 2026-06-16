# Multi-Task Run PRD

## Overview

Multi-Task Run will extend `compozy tasks run --multiple <slug-a>,<slug-b>`
with opt-in true parallel execution for independent task workflows. The primary
V1 promise is faster completion for solo developers running independent task
batches, while preserving clear per-task status and manual review handoff.

V1 keeps `enqueued` as the safe default. Users opt into parallel execution with
`--parallel` or `[tasks.run] run_multiple_mode = "parallel"`.

## Goals

- Reduce wall-clock time for independent multi-task batches.
- Make the existing `parallel` mode truthful instead of falling back to enqueued
  execution.
- Preserve existing single-task behavior for `compozy tasks run <slug>`.
- Preserve all child worktrees for manual review after a parallel batch.
- Show every child task's final status and worktree path.

## User Stories

- As a solo developer, I want `compozy tasks run --multiple a,b --parallel` so
  that independent tasks can run at the same time.
- As a solo developer, I want each child worktree preserved so that I can review
  results manually.
- As a cautious operator, I want enqueued mode to remain the default so that
  parallel execution is always explicit.
- As an existing user, I want current `tasks run --multiple` behavior to remain
  stable unless I opt into parallel mode.

## Core Features

### Explicit Parallel Mode

Users can enable parallel execution for a multi-run invocation with
`--parallel`. Configured `run_multiple_mode = "parallel"` also enables parallel
behavior.

### Safe Default

`run_multiple_mode = "enqueued"` remains the default. Existing `tasks run` and
non-parallel `--multiple` usage remain unchanged.

### Preserved Child Worktrees

Each child task run produces a preserved worktree for manual review. V1 does not
delete successful child worktrees automatically.

### Conservative Concurrency

Parallel mode uses a conservative concurrency cap. The exact number belongs in
the TechSpec, but V1 must avoid unbounded fanout.

### Fail-Late Aggregate Result

If one child fails, sibling tasks continue. The parent run reports aggregate
failure after all active children finish if any child failed, was canceled, or
could not start.

### Clear Final Handoff

At the end of a parallel run, Compozy shows each requested slug, final status,
and child worktree path.

## User Experience

1. The user runs `compozy tasks run --multiple alpha,beta --parallel`.
2. Compozy starts a parent multi-run and runs child task workflows in parallel up
   to the concurrency cap.
3. The TUI or stream output shows per-task status as children run.
4. If one child fails, other children keep running.
5. When the batch ends, Compozy shows an aggregate result plus each child status
   and worktree path.
6. The user reviews each preserved worktree manually.

## High-Level Technical Constraints

- Parallel mode must use isolated git worktrees for child task work.
- Worktrees isolate git working state, not ports, credentials, caches, provider
  limits, databases, or merge conflicts.
- Existing parent/child run accounting and tabbed visibility should remain the
  user-facing model.
- V1 must not require users to learn a new dashboard or workflow manager.

## Non-Goals

- Auto-merge or auto-push.
- Conflict prediction or conflict hints.
- Dependency-aware scheduling.
- User-configurable concurrency in V1.
- Automatic cleanup of successful child worktrees.
- Full sandboxing beyond git working-state isolation.
- A new multi-agent dashboard.

## Phased Rollout Plan

### MVP: Speed-First Parallel Mode

- Add explicit parallel mode for `tasks run --multiple`.
- Preserve all child worktrees.
- Show each child status and worktree path.
- Continue siblings after child failure.
- Report aggregate parent failure when any child fails.

Success criteria: independent task batches complete faster and remain reviewable.

### Phase 2: Review Guidance

- Add suggested manual review order.
- Improve mixed-success summaries.
- Add cleanup guidance after review.

Success criteria: users can act on parallel results with less manual
bookkeeping.

### Phase 3: Managed Integration

- Explore conflict hints, cleanup automation, and richer integration flows.

Success criteria: parallel task output becomes easier to reconcile without
hiding risk.

## Success Metrics

| Metric | Target |
| --- | --- |
| Wall-clock reduction | >= 40% faster for 3 independent tasks vs enqueued mode |
| Status clarity | 100% requested child tasks show final status |
| Handoff clarity | 100% child tasks show worktree path |
| Failure semantics | 100% sibling tasks continue after one child failure |
| Worktree preservation | 100% child worktrees remain available after run completion |

## Risks and Mitigations

- **False independence**: Users may parallelize tasks that conflict. Mitigate by
  positioning V1 for independent tasks and preserving worktrees for review.
- **Review burden**: More parallel output can create manual review work.
  Mitigate by showing status and paths clearly.
- **Resource pressure**: Too many agents can overload local/provider resources.
  Mitigate with a conservative cap.
- **Expectation mismatch**: Users may expect full sandboxing. Mitigate by
  documenting what worktrees do and do not isolate.

## Architecture Decision Records

- [ADR-001: Model Multi-Task Run as Explicit Run Orchestration](adrs/adr-001.md)
  - Treat multi-task execution as orchestration over independent child task runs.
- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md)
  - Preserve single-run behavior while creating a distinct multi-run product
    surface.
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md)
  - Use `run_multiple_mode` and preserve forward compatibility for `parallel`.
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md)
  - Keep multi-run lifecycle durable under a daemon-owned parent run.
- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md)
  - Make true parallel mode explicit, bounded, worktree-backed, and
    aggregate-fail-late.
- [ADR-006: Use a Speed-First Parallel MVP for the PRD Scope](adrs/adr-006.md)
  - Center V1 on faster independent batches, preserved worktrees, and clear
    final status handoff.

## Open Questions

- What exact concurrency cap should TechSpec use?
- What final output wording should distinguish aggregate failure from child
  failure?
- Should Phase 2 include cleanup commands, review-order hints, or both?
