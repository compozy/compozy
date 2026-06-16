---
status: completed
title: Add Git Worktree Allocation and Path Planning
type: backend
complexity: high
dependencies:
  - task_04
---

# Task 5: Add Git Worktree Allocation and Path Planning

## Overview

This task adds the git worktree allocation boundary used by parallel child runs. It resolves the current named branch and commit once, creates detached worktrees at short home-scoped paths, and returns metadata for later scheduler and snapshot use.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The allocator MUST resolve the parent workspace current branch and `HEAD` commit once per parent run.
- The allocator MUST reject a detached parent checkout with a clear error.
- The allocator MUST create detached child worktrees at the resolved base commit.
- The allocator MUST generate short deterministic paths outside the tracked repository tree.
- The allocator MUST return path, base branch, base commit, and preservation status metadata.
- The allocator MUST NOT live under `internal/core/run/internal/worktree`, because daemon code cannot import that package.
- The allocator MUST NOT create branches, merge, push, or delete worktrees in V1.
</requirements>

## Subtasks

- [x] 5.1 Add an importable worktree allocator or daemon-local allocator boundary.
- [x] 5.2 Add short deterministic path planning under the Compozy home state directory.
- [x] 5.3 Resolve named branch and base commit with detached checkout rejection.
- [x] 5.4 Create detached worktrees at the resolved commit.
- [x] 5.5 Return metadata for event and snapshot fields.
- [x] 5.6 Add disposable git repository tests.

## Implementation Details

Use the TechSpec "Worktree Lifecycle" and "Path Strategy" sections. Keep this allocator separate from the existing snapshot-only worktree package under `internal/core/run/internal/worktree`.

### Relevant Files

- `internal/config/home.go` — defines the Compozy home and state directory layout.
- `internal/core/run/internal/worktree/snapshot.go` — existing fingerprint package to avoid overloading.
- `internal/daemon/review_watch_git.go` — example narrow git boundary.
- `internal/daemon/review_watch_git_test.go` — git boundary test patterns.
- `internal/daemon/review_watch_test.go` — disposable git repository helper patterns.

### Dependent Files

- `internal/daemon/task_multi.go` — later tasks call the allocator before child launch.
- `internal/store/globaldb/registry.go` — later task resolves worktree paths as workspaces.
- `pkg/compozy/events/kinds/task.go` — metadata fields from task_04 receive allocator output.
- `README.md` — later documentation describes preserved worktree paths.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Requires per-child git worktrees.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Requires metadata before child launch.

## Deliverables

- Worktree allocator and path planner with tested metadata output.
- Detached parent checkout validation.
- Real git worktree creation tests in temp repositories.
- Unit tests with 80%+ coverage for path planning and validation helpers **(REQUIRED)**.
- Integration tests for `git worktree add --detach` behavior **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Path planner returns deterministic parent-run-scoped paths.
  - [x] Path planner sanitizes slugs containing spaces or path separators.
  - [x] Path planner keeps paths short enough for local daemon/socket constraints.
  - [x] Detached checkout detection returns a clear validation error.
  - [x] Existing target path collision returns a clear error.
- Integration tests:
  - [x] Allocator resolves the current branch and `HEAD` in a temp git repository.
  - [x] Allocator creates a detached worktree at the resolved commit.
  - [x] Created worktree has the expected `HEAD` commit.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Parallel scheduler code has a tested worktree allocator boundary.
- Allocated worktrees are short-path, detached, and preserved.
