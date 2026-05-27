# Multi-Task Run PRD

## Overview

Multi-Task Run lets a solo developer start several task workflows from one
command without changing existing `compozy tasks run <slug>` behavior. V1
introduces `compozy tasks run-multiple`, accepts multiple workflow slugs, follows
`[tasks.run] run_multiple_mode`, defaults to `enqueued`, and shows the familiar
task-run TUI with one tab per requested task.

The feature solves repeated command entry while protecting the stable single-task
runner. Parallel execution is intentionally deferred because concurrent agents
need git worktree isolation before they can safely edit files at the same time.

## Goals

- Reduce multi-workflow startup from repeated commands to one command.
- Preserve current `compozy tasks run <slug>` behavior.
- Use `run_multiple_mode` as the config source of truth for multi-run behavior.
- Default to safe enqueued execution.
- Show queued, running, completed, and failed tasks as TUI tabs.
- Accept `parallel` in V1 but fall back to `enqueued` with a message that
  parallel worktree-backed execution is planned for V2.

## User Stories

- As a solo developer, I want `compozy tasks run-multiple` so that I can start
  several workflows without repeating flags.
- As a solo developer, I want one tab per requested task so that queued and
  active workflows stay visible in the familiar TUI.
- As a cautious operator, I want enqueued execution by default so that agents do
  not edit the same checkout concurrently.
- As an existing Compozy user, I want `compozy tasks run <slug>` to remain stable
  so that my current habits, scripts, and documentation keep working.

## Core Features

### Dedicated Multi-Run Command

V1 adds `compozy tasks run-multiple` for multi-task workflows. The existing
`compozy tasks run <slug>` command remains the canonical single-workflow runner.

### Multiple Slug Input

The command accepts multiple task workflow slugs in one invocation. The expected
input shape is a comma-separated list of workflow slugs.

### Config-Driven Mode

`[tasks.run] run_multiple_mode` controls the scheduling mode. The default value
is `enqueued`.

### Enqueued Execution

V1 runs requested task workflows in order. Later tasks remain queued until the
current task finishes.

### Parallel Fallback Messaging

If `run_multiple_mode` is configured as `parallel` in V1, Compozy accepts the
configuration but falls back to enqueued execution. The user sees a clear message
that parallel mode is planned for V2 with git worktree isolation.

### Tabbed TUI

The task-run TUI shows one tab for each requested workflow. Tabs exist for queued
tasks before they start, then remain available as tasks run and finish.

### Per-Task Visibility

Each tab identifies the workflow slug and shows whether the task is queued,
running, completed, failed, or not started.

## User Experience

Primary flow:

1. The user runs `compozy tasks run-multiple` with multiple task workflow slugs.
2. Compozy reads `run_multiple_mode` and uses `enqueued` unless configured
   otherwise.
3. If the configured mode is `parallel`, Compozy displays the V2 fallback message
   and proceeds in enqueued mode.
4. The TUI opens with one tab per requested workflow.
5. The first workflow runs while later workflows remain visible as queued tabs.
6. The user can switch tabs to inspect each workflow's state.
7. As each workflow finishes, the next queued workflow starts.
8. The final state makes it clear which workflows completed, failed, or did not
   start.

The MVP should feel like the current task-run experience, not a new dashboard.
Tabs are the primary new interaction.

## High-Level Technical Constraints

- Existing `compozy tasks run <slug>` behavior must remain unchanged.
- Parallel execution is not part of V1 because safe concurrent agent execution
  requires git worktree isolation.
- The product must preserve per-workflow visibility so users know which slug maps
  to which state.
- `run_multiple_mode` should remain forward-compatible with V2 parallel support.

## Non-Goals

- Real parallel execution in V1.
- Git worktree orchestration in V1.
- Dependency graph scheduling.
- A rich batch dashboard beyond tabbed task separation.
- Changing the existing `compozy tasks run <slug>` command behavior.
- Named task groups.

## Phased Rollout Plan

### MVP: Dedicated Enqueued Multi-Run

- Add the dedicated `tasks run-multiple` user flow.
- Support comma-separated task workflow slugs.
- Default `run_multiple_mode` to `enqueued`.
- Accept `parallel` but fall back to enqueued mode with a V2 message.
- Add tabbed TUI separation for every requested workflow.
- Keep `tasks run` stable.

Success criteria: users can start multiple workflows from one command and track
each workflow through tabs without changing the current single-run command.

### Phase 2: Better Multi-Run Controls

- Improve aggregate summaries and recovery affordances.
- Refine documentation and examples for `tasks run` vs `tasks run-multiple`.
- Clarify partial success, failure, and retry paths.

Success criteria: users can understand partial success, failure, and recovery
paths without reading raw logs.

### Phase 3: Parallel With Isolation

- Add parallel execution only after git worktree isolation is available.
- Make parallel behavior explicit and safe.
- Keep enqueued mode as the safe default.

Success criteria: multiple agents can run concurrently without editing the same
checkout.

## Success Metrics

| Metric | Target |
| --- | --- |
| Command reduction | 3 task workflows can start from 1 command |
| Existing runner stability | Existing `tasks run` behavior remains unchanged |
| Default safety | `run_multiple_mode` defaults to `enqueued` |
| Task visibility | 100% of requested slugs appear as TUI tabs |
| Parallel fallback clarity | 100% of V1 `parallel` uses show fallback messaging |

## Risks and Mitigations

- **Discoverability risk**: Users may not find the new command. Mitigate with
  examples near `tasks run` help and documentation.
- **Parallel expectation risk**: Users may expect `parallel` to run concurrently
  immediately. Mitigate with clear fallback messaging and V2 documentation.
- **UI complexity risk**: Tabs may make a simple feature feel larger than needed.
  Mitigate by keeping the TUI familiar and limiting V1 to tabs plus clear state.
- **Naming risk**: `run-multiple` adds a second command beside `run`. Mitigate by
  documenting the distinction clearly: `run` for one workflow, `run-multiple` for
  several workflows.

## Architecture Decision Records

- [ADR-001: Model Multi-Task Run as Explicit Run Orchestration](adrs/adr-001.md) — Treat multi-task execution as orchestration over independent task runs.
- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md) — Protect `tasks run` by adding a dedicated multi-run command.
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md) — Fix the V1 command name, config key, parallel fallback, and tab behavior.

## Open Questions

None for PRD scope. The TechSpec should decide exact validation behavior and
fallback message copy.
