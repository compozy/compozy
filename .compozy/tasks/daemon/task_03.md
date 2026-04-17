---
status: pending
title: Run DB and Durable Run Store
type: backend
complexity: high
dependencies:
  - task_01
---

# Run DB and Durable Run Store

## Overview
This task moves per-run operational state from workspace-local JSON and JSONL artifacts into `~/.compozy/runs/<run-id>/run.db`. It defines the durable run store, writer-loop semantics, and filesystem allocation rules that later transport, reconciliation, and public-reader migration tasks will consume.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "run.db", "Run Lifecycle and Recovery", and "Known Risks" instead of duplicating them here
- FOCUS ON "WHAT" — keep deterministic event ordering and stable run identity while changing the backing store
- MINIMIZE CODE — replace the persistence seam, not the execution engine that feeds it
- TESTS REQUIRED — unit and integration coverage are mandatory for run store behavior
</critical>

<requirements>
1. MUST allocate `~/.compozy/runs/<run-id>/run.db` before external execution starts.
2. MUST create `run.db` with `schema_migrations` bookkeeping and the tables defined in the TechSpec.
3. MUST replace direct JSONL append semantics with serialized run-store writes that preserve event order and replay cursors.
4. MUST persist events, job state, transcript rows, hook audit data, token usage, and artifact sync history in the new run store.
5. SHOULD preserve current run ID generation semantics for task, review, and exec modes unless the TechSpec explicitly changes them.
</requirements>

## Subtasks
- [ ] 3.1 Define the `run.db` schema and migration bookkeeping.
- [ ] 3.2 Introduce a serialized writer loop for run-scoped durable writes.
- [ ] 3.3 Move run artifact allocation from workspace-local paths to the home-scoped run directory.
- [ ] 3.4 Persist the canonical event stream and projection tables needed for later snapshots and replay.
- [ ] 3.5 Add tests covering ordering, allocation, and migration idempotence.

## Implementation Details
Implement the per-run storage layer described in the TechSpec "run.db", "Run Lifecycle and Recovery", and "Build Order" sections. This task should focus on durable storage and ordering guarantees, while leaving API exposure and reconciliation semantics to later tasks.

### AGH Reference Files
- `~/dev/compozy/agh/internal/store/sessiondb/session_db.go` — reference for the per-session DB split, writer ownership, and event-storage helpers.
- `~/dev/compozy/agh/internal/session/manager.go` — reference for how per-session storage and lifecycle are owned together.

### Relevant Files
- `internal/core/model/run_scope.go` — current run artifact allocation and run ID generation seam.
- `internal/core/run/journal/journal.go` — existing JSONL journal that must be replaced with a durable writer-loop contract.
- `internal/core/model/artifacts.go` — current run artifact layout assumptions tied to workspace-local files.
- `internal/core/run/executor/event_stream.go` — current event flow that will later feed the new run store.
- `pkg/compozy/runs/layout/layout.go` — public layout contract that currently assumes workspace-local run artifacts.
- `internal/store/rundb/run_db.go` — new durable run store implementation.
- `internal/store/rundb/migrations.go` — new schema and migration bookkeeping for `run.db`.

### Dependent Files
- `internal/core/run/executor/execution.go` — later execution work will need terminal state mirrored into the durable run store.
- `internal/api/core/handlers.go` — later snapshots and event pagination will depend on the new run-store tables.
- `internal/daemon/reconcile.go` — startup reconciliation will inspect durable run state introduced here.
- `pkg/compozy/runs/run.go` — public run readers will later migrate off workspace-local files onto daemon-backed reads of this durable state.

### Related ADRs
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — defines `run.db` as the per-run operational store.

## Deliverables
- `run.db` schema and migration bookkeeping.
- Serialized run-scoped durable writer loop.
- Home-scoped run directory allocation for all run modes.
- Unit tests with 80%+ coverage for ordering and migration behavior **(REQUIRED)**
- Integration tests covering durable run allocation and replay-ready event persistence **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Applying `run.db` migrations repeatedly leaves the schema unchanged and migration history consistent.
  - [ ] Events written through the run-store writer loop keep monotonically increasing sequence order under concurrent publishers.
  - [ ] Transcript, hook audit, token usage, and artifact sync log rows round-trip through the run store without schema loss.
  - [ ] Auto-generated and explicit `run_id` values allocate the correct home-scoped run directory paths.
  - [ ] Closing the run store flushes pending writes before the writer loop exits.
- Integration tests:
  - [ ] Starting a run creates `~/.compozy/runs/<run-id>/run.db` before execution begins.
  - [ ] A persisted run can be reopened for later snapshot and replay work without depending on workspace-local `events.jsonl`.
  - [ ] Concurrent event producers for one run are serialized into one deterministic event sequence in `run.db`.
  - [ ] A run with transcript, hook, and token records can be reopened after daemon restart with the same durable data available for later snapshot assembly.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Per-run state is allocated under `~/.compozy/runs/<run-id>/`
- Durable event ordering matches the old journal guarantees
- `run.db` is migration-safe and replay-ready for later transport tasks
