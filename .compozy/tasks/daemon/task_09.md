---
status: pending
title: Archive Rewrite on DB State
type: refactor
complexity: medium
dependencies:
  - task_07
  - task_08
---

# Archive Rewrite on DB State

## Overview
This task rewrites workflow archiving to depend on synced database state instead of generated `_meta.md` files. It preserves the current archive intent while making eligibility, active-run conflicts, and archived naming deterministic under the daemon model.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Sync and Archive Semantics" and "Task workflows" instead of duplicating them here
- FOCUS ON "WHAT" — archive policy should reflect synced workflow truth, not stale filesystem metadata
- MINIMIZE CODE — keep the archive operation narrow and reuse synced task/review state instead of reparsing everything ad hoc
- TESTS REQUIRED — unit and integration coverage are mandatory for eligibility, conflicts, and archive naming
</critical>

<requirements>
1. MUST determine archive eligibility from synced task and review state in `global.db`, not from `_meta.md` files.
2. MUST reject archive attempts with `409` when the workflow still has an active run.
3. MUST preserve the archive move behavior to `.compozy/tasks/_archived/<timestamp-ms>-<shortid>-<slug>`.
4. MUST keep incomplete tasks or unresolved review issues from being archived.
5. SHOULD sort and report skipped workflows deterministically for workspace-wide archive operations.
</requirements>

## Subtasks
- [ ] 9.1 Replace metadata-file-based archive eligibility checks with DB-backed task and review state queries.
- [ ] 9.2 Add active-run conflict handling for per-workflow and workspace-wide archive operations.
- [ ] 9.3 Update archived directory naming to the new timestamp-plus-shortid format from the TechSpec.
- [ ] 9.4 Keep archive result reporting deterministic for archived and skipped workflows.
- [ ] 9.5 Add tests covering completed, incomplete, unresolved-review, and active-run conflict cases.

## Implementation Details
Implement the archive behavior described in the TechSpec "Sync and Archive Semantics" and "Task workflows" sections. This task should remove the last operational dependency on workflow `_meta.md` files for archive eligibility while preserving the current archive directory move semantics users already expect.

### Relevant Files
- `internal/core/archive.go` — current archive flow that still depends on `_meta.md` and review metadata files.
- `internal/core/tasks/store.go` — current task discovery helpers that archive must stop treating as the operational source of completion state.
- `internal/core/reviews/store.go` — review round discovery and status helpers that should now feed DB-backed archive checks.
- `internal/store/globaldb/archive.go` — new archive eligibility queries and workflow archival persistence.
- `internal/core/model/workspace_paths.go` — archive destination naming and path helpers.

### Dependent Files
- `internal/api/core/handlers.go` — archive routes and conflict responses depend on the DB-backed eligibility rules introduced here.
- `internal/cli/commands.go` — top-level archive command behavior will surface the new conflict and skip semantics.
- `internal/core/sync.go` — archive depends on synced workflow state being current and consistent.

### Related ADRs
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — moves archive eligibility from generated metadata to operational DB state.

## Deliverables
- DB-backed archive eligibility rules for workflows and review rounds.
- Active-run conflict behavior and updated archive naming.
- Deterministic archive result reporting for single and workspace-wide archive requests.
- Unit tests with 80%+ coverage for archive eligibility and naming **(REQUIRED)**
- Integration tests covering archive conflicts and completed workflow moves **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] A workflow with pending tasks is reported as non-archivable even if legacy metadata files are missing or stale.
  - [ ] A workflow with unresolved review issues is skipped with the expected reason from DB-backed state.
  - [ ] Archived directory names include the timestamp, short ID, and slug in the expected order.
- Integration tests:
  - [ ] `compozy archive --name <slug>` archives a fully completed workflow into the new archived path format.
  - [ ] `POST /tasks/:slug/archive` returns `409` when the workflow still has an active run.
  - [ ] Workspace-wide archive skips incomplete workflows deterministically and archives only the eligible ones.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Archive eligibility comes entirely from synced DB state
- Active runs block archive with explicit conflict semantics
- Completed workflows move into the new archived path format deterministically
