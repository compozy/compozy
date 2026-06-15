# Multi-Task Run

## Overview

Extend `compozy tasks run --multiple <slug-a>,<slug-b>` with true opt-in
parallel execution for independent task workflows. V1 adds `--parallel` and
honors `[tasks.run] run_multiple_mode = "parallel"` by running child task runs
concurrently in isolated git worktrees. The feature is for solo developers who
want faster batches without agents editing the same checkout.

## Problem

Today, `--multiple` reduces repeated commands but still runs task workflows
sequentially. Config already accepts `parallel`, but Compozy downgrades it to
enqueued execution, which makes the configuration less trustworthy and leaves
the fastest independent-task workflow unsolved.

Parallel AI coding agents need filesystem isolation. Git worktrees provide
separate working directories, `HEAD`, and indexes while sharing repository
objects. They prevent same-checkout file and git-index collisions, but they do
not isolate ports, credentials, caches, provider limits, databases, or merge
conflicts.

### Market Data

Nx exposes task parallelism through CLI/config (`--parallel` and `nx.json`
defaults). Turborepo supports running one or many tasks through a single command.
AI coding workflows increasingly use one worktree per agent because shared
checkouts create overwrites, stale context, and git lock contention. Recent
worktree tooling comparisons also warn that worktrees are necessary but not
sufficient: recombination, shared runtime state, and review still need explicit
boundaries.

## Summary / Differentiator

Compozy can combine task-run accounting with worktree-backed agent isolation:
one parent run, many child runs, clear tabs/status, and explicit manual
integration instead of an opaque shell fan-out.

## Core Features

| #   | Feature                    | Priority | Description |
| --- | -------------------------- | -------- | ----------- |
| F1  | `--parallel` CLI override  | Critical | Add `compozy tasks run --multiple a,b --parallel` as an explicit per-invocation mode. |
| F2  | Config-backed parallel mode | Critical | Honor `[tasks.run] run_multiple_mode = "parallel"` instead of downgrading it. |
| F3  | Per-child git worktrees    | Critical | Allocate one worktree per child task run and remap child workspace/task paths into it. |
| F4  | Bounded concurrency        | Critical | Run children concurrently up to a conservative cap to avoid local/provider overload. |
| F5  | Fail-late aggregation      | Critical | Let siblings continue after one child fails; fail the parent if any child fails or is canceled. |
| F6  | Durable worktree metadata  | High     | Persist branch/path/base/cleanup status for snapshots, recovery, and manual review. |
| F7  | Existing tab/status reuse  | High     | Keep parent `task_multi`, child runs, snapshots, events, and TUI tabs as the user surface. |

## Integration with Existing Features

| Integration Point              | How |
| ------------------------------ | --- |
| `tasks run --multiple`         | Adds true parallel scheduling to the existing multi-run path. |
| `[tasks.run] run_multiple_mode` | Keeps `enqueued` as default and makes `parallel` truthful. |
| `task_multi` parent runs       | Reuses parent/child accounting while adding a distinct parallel coordinator. |
| Multi-run TUI                  | Reuses tabs and per-child status; no new dashboard in V1. |

## KPIs

| KPI                          | Target                                             | How to Measure |
| ---------------------------- | -------------------------------------------------- | -------------- |
| Wall-clock reduction         | >= 40% faster for 3 independent tasks vs enqueued mode | Integration benchmark with stub or real child runtimes |
| Isolation correctness        | 100% child runs use distinct worktree paths and branches | Daemon/API tests and snapshot assertions |
| Status attribution           | 100% requested slugs have final child status and run ID | Multi-run snapshot and TUI tests |
| Failure semantics            | 100% sibling runs continue after one child failure | Coordinator test with one failing child |
| Changed worktree preservation | 100% changed child worktrees are preserved for manual integration | Cleanup-policy tests |

## Feature Assessment

| Criteria            | Question                                            | Score  |
| ------------------- | --------------------------------------------------- | ------ |
| **Impact**          | How much more valuable does this make the product?  | Strong |
| **Reach**           | What % of users would this affect?                  | Strong |
| **Frequency**       | How often would users encounter this value?         | Strong |
| **Differentiation** | Does this set us apart or just match competitors?   | Strong |
| **Defensibility**   | Is this easy to copy or does it compound over time? | Maybe  |
| **Feasibility**     | Can we actually build this?                         | Strong |

Leverage type: Quick Win.

## Council Insights

- **Recommended approach:** Ship true parallel mode as an explicit, bounded,
  worktree-backed child execution mode under the existing `task_multi` parent.
- **Key trade-offs:** Faster batches vs manual merge burden; minimal scope vs
  lifecycle discipline; worktree isolation vs remaining shared runtime
  resources.
- **Risks identified:** False task independence, orphaned worktrees, shared
  ports/caches/provider limits, confusing mixed success/failure states.
- **Stretch goal (V2+):** Add conflict prediction, guided integration, and
  richer review/cleanup flows.

## Out of Scope (V1)

- **Auto-merge or auto-push** — V1 preserves changed worktrees for manual review.
- **Dependency-aware scheduling** — V1 assumes the user selected independent
  tasks.
- **Full sandboxing** — Worktrees isolate git working state, not ports,
  databases, credentials, or provider limits.
- **Conflict prediction** — Useful later, but not required to prove parallel
  batch value.
- **New multi-agent dashboard** — Existing snapshots and tabs are enough for V1.

## Architecture Decision Records

- [ADR-001: Model Multi-Task Run as Explicit Run Orchestration](adrs/adr-001.md)
- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md)
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md)
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md)
- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md)

## Open Questions

- What should the initial concurrency cap be: fixed `2`, fixed `3`, or
  configurable in V1?
- What exact branch/path naming pattern should child worktrees use?
- Should unchanged successful worktrees be removed automatically, or should V1
  preserve all child worktrees?
- How should Compozy surface manual integration instructions at the end of a
  mixed-success run?

## References

- Git worktree docs: https://git-scm.com/docs/git-worktree
- Nx parallel tasks: https://nx.dev/docs/guides/tasks--caching/run-tasks-in-parallel
- Turborepo run docs: https://turborepo.dev/docs/reference/run
- Augment guide on worktrees for parallel agents: https://www.augmentcode.com/guides/git-worktrees-parallel-ai-agent-execution
- Nimbalyst 2026 worktree tooling comparison: https://nimbalyst.com/blog/best-git-worktree-tools-ai-coding-2026/
