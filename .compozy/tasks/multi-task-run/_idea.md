# Multi-Task Run

## Overview

Allow `compozy tasks run` to accept a comma-separated list of workflow slugs, such as
`compozy tasks run task_a,task_b --ide codex`, so a solo developer can start multiple
independent task workflows with one command. V1 keeps each slug as its own daemon task
run and adds `[tasks.run]` config to choose enqueued or parallel scheduling.

## Problem

Today, running more than one task workflow requires repeated commands:

```bash
compozy tasks run task_a --ide codex
compozy tasks run task_b --ide codex
```

That is unnecessary friction for solo developers working through independent task
bundles. The user must repeat flags, manually track which commands were started, and
coordinate ordering or parallelism outside Compozy.

The feature should reduce repeated CLI work without weakening Compozy's run accounting.
A comma-separated invocation should still make it clear which slug produced which run,
which runs started, which failed, and how to resume or inspect each child run.

### Market Data

Task runners such as Nx and Turborepo support multi-target execution with configurable
parallelism. Tools like `concurrently` exist because repeated terminals and shell
backgrounding make status and failure tracking hard. AI coding workflows are also moving
toward parallel agent sessions, usually with explicit isolation or worktree guidance.

## Core Features

| #   | Feature                       | Priority | Description                                                                 |
| --- | ----------------------------- | -------- | --------------------------------------------------------------------------- |
| F1  | Comma-separated task slugs    | Critical | Accept `task_a,task_b` as one positional input while preserving current single-slug behavior. |
| F2  | Config scheduling mode        | Critical | Add `[tasks.run]` config for enqueued vs parallel execution.                 |
| F3  | Independent daemon runs       | Critical | Start one existing daemon task run per slug instead of creating a merged execution model. |
| F4  | Shared runtime flags          | High     | Apply shared CLI flags such as `--ide`, `--model`, and `--include-completed` consistently to every child run. |
| F5  | Aggregate reporting           | High     | Print each slug, run ID, startup/result status, and final aggregate outcome. |

## KPIs

| KPI                    | Target                                | How to Measure                         |
| ---------------------- | ------------------------------------- | -------------------------------------- |
| Command reduction      | 3 workflows start with 1 command instead of 3 | CLI invocation comparison in docs/tests |
| Slug accounting        | 100% of requested slugs appear in output | CLI integration tests                  |
| Backward compatibility | 100% existing single-slug tests pass unchanged | Regression suite                       |
| Queued determinism     | 100% queued runs start in input order | Integration test with stub daemon client |
| Parallel support       | At least 2 runs can be started concurrently | Integration test proving overlapping start calls |

## Feature Assessment

| Criteria            | Question                                            | Score  |
| ------------------- | --------------------------------------------------- | ------ |
| **Impact**          | How much more valuable does this make the product?  | Strong |
| **Reach**           | What % of users would this affect?                  | Strong |
| **Frequency**       | How often would users encounter this value?         | Strong |
| **Differentiation** | Does this set us apart or just match competitors?   | Maybe  |
| **Defensibility**   | Is this easy to copy or does it compound over time? | Maybe  |
| **Feasibility**     | Can we actually build this?                         | Strong |

Leverage type: Quick Win.

## Council Insights

- **Recommended approach:** Ship a thin orchestration layer over existing single-task daemon runs.
- **Key trade-offs:** Fewer commands vs clearer run semantics; parallel speed vs shared-workspace conflict risk; simple CLI output vs richer batch UI.
- **Risks identified:** Ambiguous partial failures, cancellation surprises, and parallel agents editing the same checkout.
- **Stretch goal (V2+):** Add isolated worktree-backed parallel execution or a richer multi-run dashboard.

## Out of Scope (V1)

- **Dependency graph scheduling** — V1 does not infer relationships between task workflows.
- **One aggregate "super run"** — each slug remains its own daemon run.
- **Worktree isolation** — parallel mode shares the current workspace unless a later feature changes that.
- **Multi-run TUI dashboard** — V1 should use conservative output and attachment behavior.
- **Named task groups** — useful later, but not required for comma-separated execution.

## Architecture Decision Records

- [ADR-001: Model Multi-Task Run as Explicit Run Orchestration](adrs/adr-001.md) — Treat comma-separated slugs as orchestration over independent daemon runs.

## Open Questions

- What exact config key should represent scheduling mode: `schedule`, `execution_mode`, or `multi_run_mode`?
- Should parallel mode have a numeric concurrency cap in V1?
- For `--stream` or UI attach, should V1 disallow multi-slug attach or stream aggregate text output only?
- Should `--name` accept comma-separated slugs or remain single-slug only?

## References

- [Nx run tasks](https://nx.dev/docs/features/run-tasks)
- [Nx commands](https://nx.dev/docs/reference/nx-commands)
- [Turborepo task configuration](https://turborepo.ai/docs/crafting-your-repository/configuring-tasks)
- [concurrently](https://www.npmjs.com/package/concurrently?activeTab=readme)
- [Claude Code worktrees](https://code.claude.com/docs/en/worktrees)
- [Cursor CLI](https://cursor.com/blog/cli)
