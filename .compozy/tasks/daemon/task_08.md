---
status: pending
title: Active-Run Watchers and Legacy Metadata Cleanup
type: refactor
complexity: high
dependencies:
  - task_05
  - task_07
---

# Active-Run Watchers and Legacy Metadata Cleanup

## Overview
This task adds the intelligent watcher behavior discussed in the design: only active runs get file watching, and only for the workflow they currently own. It also closes the migration away from generated metadata by cleaning up legacy `_tasks.md` and `_meta.md` artifacts and ensuring manual Markdown edits are reflected back into daemon state during a run.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Artifact sync service" and "Sync and Archive Semantics" instead of duplicating them here
- FOCUS ON "WHAT" — watchers exist to keep active-run state synchronized, not to become a global always-on indexing service
- MINIMIZE CODE — scope file watching tightly to one active workflow and reuse sync ingestion paths for actual parsing
- TESTS REQUIRED — unit and integration coverage are mandatory for watcher scope, debounce, and cleanup behavior
</critical>

<requirements>
1. MUST start file watchers only for the workflow owned by an active run and stop them when that run terminates.
2. MUST debounce bursty writes and persist the effective sync checkpoint used by active-run watchers.
3. MUST watch the active workflow's task, review, memory, protocol, prompt, ADR, and QA files, and ignore reparsing outside that workflow root.
4. MUST synchronize manual Markdown edits back into daemon state during the run through the same sync ingestion path.
5. MUST perform one-time legacy `_tasks.md` and `_meta.md` cleanup without recreating those files later.
</requirements>

## Subtasks
- [ ] 8.1 Add active-run watcher lifecycle management tied directly to run start and run shutdown.
- [ ] 8.2 Limit watcher scope to the active workflow root and debounce repeated writes before reparsing.
- [ ] 8.3 Route watcher-triggered updates through the sync persistence layer instead of bespoke patch logic.
- [ ] 8.4 Implement one-time cleanup or rename behavior for generated `_tasks.md` and `_meta.md`.
- [ ] 8.5 Add tests covering watcher scope, debounce, checkpoint persistence, and legacy metadata migration.

## Implementation Details
Implement the active-run watcher model described in the TechSpec "Artifact sync service" and "Sync and Archive Semantics" sections. This task should keep live workflow state aligned with Markdown edits during execution while explicitly avoiding a daemon-wide always-on watcher that would add unnecessary load and complexity.

### Relevant Files
- `internal/daemon/watchers.go` — new watcher coordinator tied to daemon-managed runs.
- `internal/core/sync.go` — existing sync entrypoint that watcher updates should reuse instead of bypassing.
- `internal/core/tasks/store.go` — current task-file discovery and store helpers touched by live re-sync behavior.
- `internal/core/reviews/store.go` — current review round discovery used by watcher-driven updates.
- `internal/core/memory/store.go` — memory artifacts must stay in sync during active runs.
- `internal/store/globaldb/sync.go` — checkpoint and artifact update persistence for watcher-driven reconciliation.

### Dependent Files
- `internal/core/model/artifacts.go` — artifact path helpers must still describe the watched workflow layout correctly.
- `internal/api/core/handlers.go` — clients will later observe watcher-driven `artifact.synced` effects through daemon queries and streams.
- `internal/core/archive.go` — archive behavior depends on legacy metadata cleanup being complete and DB state being current.

### Related ADRs
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — requires Markdown edits to flow back into operational state.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — benefits from live state updates while users keep editing artifacts locally.

## Deliverables
- Run-scoped watcher lifecycle tied to daemon-managed runs.
- Debounced re-sync behavior for active workflow artifacts.
- One-time legacy metadata cleanup for `_tasks.md` and `_meta.md`.
- Unit tests with 80%+ coverage for watcher scope and checkpoint behavior **(REQUIRED)**
- Integration tests covering live Markdown edits during an active run **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Starting a run creates a watcher only for that workflow and stopping the run tears it down cleanly.
  - [ ] Repeated writes within the debounce window collapse into one effective sync update and checkpoint.
  - [ ] Writes outside the active workflow root do not trigger reparsing or mutate synced state.
- Integration tests:
  - [ ] Editing a `task_XX.md` file during an active run updates the corresponding synced task state without restarting the run.
  - [ ] Editing a review issue or memory file during an active run updates `global.db` through the watcher path.
  - [ ] The first watcher-enabled sync cleans legacy `_tasks.md` and `_meta.md` once and never recreates them afterward.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Watchers run only when there is an active run and only for that run's workflow
- Manual Markdown edits are reflected in daemon state during execution
- Legacy generated metadata is cleaned up and stays gone
