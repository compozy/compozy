---
status: completed
title: Register and Remap Parallel Children to Worktree Workspaces
type: backend
complexity: high
dependencies:
  - task_05
  - task_06
---

# Task 7: Register and Remap Parallel Children to Worktree Workspaces

## Overview

This task connects worktree allocation to child task-run startup. It ensures each parallel child run is registered under its physical worktree workspace and that runtime paths, task sync, watchers, and extension hooks all point at the worktree.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- Each parallel child MUST allocate or receive worktree metadata before child `startRun`.
- Child workspace rows MUST be resolved from worktree roots, not the original parent workspace root.
- Child runtime config MUST set `WorkspaceRoot` to the worktree path.
- Child runtime config MUST set `TasksDir` to the slug task directory inside the worktree.
- Child runtime config MUST set `ParentRunID` to the parent `task_multi` run id.
- The parent run MUST remain registered under the original workspace.
- Worktree metadata MUST be emitted before child launch.
</requirements>

## Subtasks

- [x] 7.1 Add parallel child preparation data for worktree metadata.
- [x] 7.2 Resolve/register worktree workspace rows before child start.
- [x] 7.3 Remap child runtime workspace and task directory paths.
- [x] 7.4 Preserve parent workspace ownership and child `ParentRunID`.
- [x] 7.5 Emit worktree metadata before child launch.
- [x] 7.6 Add daemon tests for workspace identity and runtime remapping.

## Implementation Details

Use the TechSpec "Child Runtime And Workspace Ownership" section. The key invariant is that database workspace identity and runtime filesystem roots align for parallel children.

### Relevant Files

- `internal/daemon/task_multi.go` — starts child runs and emits multi-run item events.
- `internal/daemon/run_manager.go` — `startRun` stores workspace ids and opens scopes from runtime config.
- `internal/daemon/extension_bridge.go` — existing pattern for starting child task runs from remapped runtime roots.
- `internal/store/globaldb/registry.go` — resolves/registers workspace rows from filesystem paths.
- `internal/core/model/workspace_paths.go` — builds task directory paths from workspace roots.
- `internal/core/model/runtime_config.go` — contains workspace and task directory runtime fields.
- `internal/daemon/task_multi_test.go` — daemon parent/child assertions and snapshot tests.

### Dependent Files

- `internal/core/sync.go` — sync requires task targets to belong to the registered workspace root.
- `internal/core/run/executor/execution.go` — workflow execution uses `WorkspaceRoot` and `TasksDir`.
- `internal/core/extension/host_writes_test.go` — extension host run-start behavior depends on runtime workspace roots.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Requires isolated child worktree execution.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Chooses worktree workspace ownership for child rows.

## Deliverables

- Parallel child startup path with worktree workspace rows.
- Runtime remapping for child workspace root and task directory.
- Parent/child linkage preserved through `ParentRunID`.
- Metadata emitted before child launch.
- Unit tests with 80%+ coverage for remapping helpers **(REQUIRED)**.
- Integration tests proving child rows use worktree workspace ids **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Child remap sets `WorkspaceRoot` to the worktree path.
  - [x] Child remap sets `TasksDir` to `<worktree>/.compozy/tasks/<slug>`.
  - [x] Child remap preserves unrelated runtime overrides.
  - [x] Missing task directory inside a worktree returns a slug-specific error.
  - [x] Metadata event is emitted before child-started event.
- Integration tests:
  - [x] Parallel child row `WorkspaceID` matches the worktree workspace row.
  - [x] Parent row remains under the original workspace.
  - [x] Child row keeps `ParentRunID` equal to the parent run id.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Parallel children execute with workspace identity aligned to their worktree roots.
- Parent snapshots remain the canonical grouping surface.
