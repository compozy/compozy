---
status: completed
title: Global DB and Workspace Registry
type: backend
complexity: high
dependencies:
  - task_01
---

# Global DB and Workspace Registry

## Overview
This task introduces `global.db` as the durable catalog for workspaces, workflows, sync checkpoints, and the run index. It also defines the workspace registry contract that later daemon, sync, archive, and CLI tasks will use instead of relying on implicit `cwd` discovery alone.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Data Models", "Identity Rules", and "API Endpoints" instead of duplicating them here
- FOCUS ON "WHAT" — make workspace and workflow identity explicit before layering new commands on top
- MINIMIZE CODE — prefer one registry implementation with clear interfaces over multiple partial stores
- TESTS REQUIRED — unit and integration coverage are mandatory for schema and registry behavior
</critical>

<requirements>
1. MUST create `global.db` with `schema_migrations` bookkeeping and the tables defined in the TechSpec.
2. MUST normalize and store workspace roots with a unique constraint so duplicate paths collapse to one workspace row.
3. MUST provide registry operations for resolve, register, get, list, and unregister, with active-run protection on unregister.
4. MUST preserve workflow slug uniqueness within one workspace while keeping archived workflow rows queryable.
5. SHOULD make path normalization symlink-aware so daemon identity does not drift across equivalent paths.
</requirements>

## Subtasks
- [x] 2.1 Implement `global.db` schema creation and migration bookkeeping.
- [x] 2.2 Add the workspace registry service and path normalization rules.
- [x] 2.3 Persist workflow and run index identity in the global catalog.
- [x] 2.4 Add registry-level conflict handling for duplicate roots and active-run unregister attempts.
- [x] 2.5 Add tests covering migrations, normalization, and registry semantics.

## Implementation Details
Implement the durable registry layer described in the TechSpec "Data Models", "Identity Rules", and "Impact Analysis" sections. This task should stop short of run execution and transport wiring, but it must provide the durable read/write services those later tasks depend on.

### AGH Reference Files
- `~/dev/compozy/agh/internal/store/globaldb/global_db.go` — reference for global DB bootstrap, migrations, connection helpers, and durable catalog patterns.
- `~/dev/compozy/agh/internal/observe/observer.go` — reference for query surfaces over global operational state.

### Relevant Files
- `internal/core/workspace/config.go` — current workspace discovery and config merge logic that later registry resolution must coexist with.
- `internal/core/workflow_target.go` — current workflow resolution seam that will later depend on registry-backed workspace identity.
- `internal/core/model/workspace_paths.go` — existing path assumptions that need a daemon-backed replacement.
- `internal/store/globaldb/global_db.go` — new durable global catalog implementation.
- `internal/store/globaldb/migrations.go` — new schema and migration bookkeeping for `global.db`.
- `internal/store/globaldb/registry.go` — new workspace and workflow registry operations.

### Dependent Files
- `internal/core/sync.go` — later sync work will write parsed artifact state into the global catalog.
- `internal/core/archive.go` — later archive work will read workflow completion state from `global.db`.
- `internal/api/core/handlers.go` — transport handlers will depend on registry operations and run index rows.
- `internal/cli/workspace_config.go` — daemon-aware workspace resolution will need the registry semantics introduced here.

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — requires a daemon-wide workspace registry.
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — defines `global.db` as operational truth.

## Deliverables
- `global.db` schema and migration bookkeeping.
- Durable workspace registry and workflow/run index services.
- Unique normalized root handling and active-run unregister protection.
- Unit tests with 80%+ coverage for registry and migration behavior **(REQUIRED)**
- Integration tests covering path normalization and duplicate registration behavior **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Applying `global.db` migrations twice leaves the schema unchanged and the migration history consistent.
  - [x] Opening a `global.db` with a newer unsupported schema returns `schema_too_new` instead of silently proceeding.
  - [x] Registering the same workspace through canonicalized paths and symlinked paths returns one logical workspace row.
  - [x] Creating an active workflow with a slug blocks creation of a second active workflow with the same `(workspace_id, slug)` pair while still allowing archived reuse.
  - [x] Unregistering a workspace with active runs returns a conflict instead of deleting the row.
- Integration tests:
  - [x] Resolving then explicitly registering the same workspace path yields one stable workspace identity.
  - [x] Concurrent register requests for the same workspace path collapse to one durable row and one returned identity.
  - [x] Archived and active workflows under the same workspace keep distinct query behavior without slug collisions.
  - [x] Restarting the daemon after registry writes preserves workspace, workflow, and run-index visibility without rerunning registration flows.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `global.db` is created and migrated deterministically
- Workspace identity is stable across normalized paths
- Unregister and slug uniqueness rules are enforced from durable state
