---
status: completed
title: Sync Persistence Rewrite
type: backend
complexity: high
dependencies:
  - task_02
---

# Sync Persistence Rewrite

## Overview
This task converts `compozy sync` from generated metadata file refresh into database reconciliation for workflow artifacts. It becomes the ingestion path that parses Markdown artifacts, upserts structured workflow state into `global.db`, and stops relying on `_tasks.md` and `_meta.md` as operational truth.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Artifact sync service", "global.db", and "Sync and Archive Semantics" instead of duplicating them here
- FOCUS ON "WHAT" — sync is a reconciliation contract, not a formatting or file-generation command anymore
- MINIMIZE CODE — reuse existing task/review/memory parsers instead of inventing parallel Markdown readers
- TESTS REQUIRED — unit and integration coverage are mandatory for parsing, upsert behavior, and legacy metadata removal
</critical>

<requirements>
1. MUST parse workflow Markdown artifacts and upsert `artifact_snapshots`, `task_items`, `review_rounds`, `review_issues`, and `sync_checkpoints` into `global.db`.
2. MUST stop generating or refreshing `_tasks.md` and `_meta.md` as part of sync behavior.
3. MUST keep human-authored artifact files authoritative in the workspace while making synced DB rows the operational source for daemon queries.
4. MUST support syncing a single workflow or an entire workspace without changing existing artifact file content.
5. SHOULD reuse the current task, review, and memory parsing code where it already captures the needed authored structure.
</requirements>

## Subtasks
- [x] 7.1 Replace metadata-file refresh in `sync` with DB reconciliation for one workflow and whole-workspace scopes.
- [x] 7.2 Map task, review, memory, prompt, protocol, ADR, and QA artifacts into structured snapshot rows.
- [x] 7.3 Persist checksums, mtimes, and sync checkpoints needed for later watcher-driven updates.
- [x] 7.4 Remove sync-time writes to generated `_tasks.md` and `_meta.md` artifacts.
- [x] 7.5 Add tests covering single-workflow sync, workspace-wide sync, idempotent re-sync, and legacy metadata cleanup signaling.

## Implementation Details
Implement the reconciliation model described in the TechSpec "Artifact sync service", "global.db", and "Sync and Archive Semantics" sections. This task should convert sync into a storage-backed import path that feeds daemon queries without changing the flexible Markdown-authoring workflow users have today.

### AGH Reference Files
- `~/dev/compozy/agh/internal/store/globaldb/global_db.go` — reference for projection tables, migrations, and central operational storage helpers.
- `~/dev/compozy/agh/internal/observe/observer.go` — reference for querying and projecting synced state for later clients.

### Relevant Files
- `internal/core/sync.go` — current metadata-refresh logic that must become DB reconciliation.
- `internal/core/tasks/parser.go` — existing task-file parsing that should feed `task_items` and snapshots.
- `internal/core/reviews/parser.go` — existing review parsing for round and issue ingestion.
- `internal/core/memory/store.go` — current memory-file handling that must be represented in artifact snapshots.
- `internal/store/globaldb/sync.go` — new sync persistence adapter and checkpoint writer.
- `internal/store/globaldb/workflows.go` — new workflow snapshot upsert logic for synced artifact state.

### Dependent Files
- `internal/core/archive.go` — archive eligibility must later consume synced DB state instead of `_meta.md`.
- `internal/api/core/handlers.go` — task, review, and sync routes will query the state written by this task.
- `internal/cli/validate_tasks.go` — task validation flows should remain compatible with the same authored Markdown files that sync now ingests.

### Related ADRs
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — defines the artifact-authoritative plus DB-operational split for sync.

## Deliverables
- DB-backed sync flow for workflow artifacts and checkpoints.
- Structured artifact snapshot ingestion for tasks, reviews, memory, and related workflow docs.
- Removal of sync-time `_tasks.md` and `_meta.md` generation.
- Unit tests with 80%+ coverage for sync parsing and upsert behavior **(REQUIRED)**
- Integration tests covering single-workflow and workspace-wide reconciliation **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Syncing a workflow upserts task rows, review summaries, and artifact snapshots using stable checksums and source paths.
  - [x] Re-running sync without content changes leaves snapshot checksums stable and updates only the expected checkpoint fields.
  - [x] Removing an authored artifact deletes or invalidates the corresponding synced snapshot rows instead of leaving stale state behind.
  - [x] Oversized artifact bodies follow the configured overflow strategy instead of bloating `artifact_snapshots.body_text`.
  - [x] Legacy `_tasks.md` and `_meta.md` files are no longer written as part of sync behavior.
- Integration tests:
  - [x] `compozy sync --name <slug>` ingests one workflow into `global.db` without mutating authored artifact files.
  - [x] Workspace-wide sync discovers all active workflows and persists their structured state into `global.db`.
  - [x] Editing task Markdown and re-running sync updates the existing workflow row instead of creating duplicate task identity.
  - [x] A workflow with review rounds, memory files, prompts, protocol docs, and QA artifacts produces consistent snapshot and projection rows.
  - [x] The first sync of a legacy workflow records one cleanup warning while removing generated `_tasks.md` and `_meta.md` artifacts.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Sync writes operational state to `global.db` instead of regenerating metadata files
- Human-authored Markdown remains flexible while daemon queries gain structured workflow state
- Single-workflow and whole-workspace sync are deterministic and idempotent
