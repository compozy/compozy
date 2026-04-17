---
status: pending
title: pkg/compozy/runs Daemon-Backed Migration
type: refactor
complexity: high
dependencies:
  - task_04
  - task_05
---

# pkg/compozy/runs Daemon-Backed Migration

## Overview
This task migrates the public run-reader package away from workspace-local filesystem assumptions and onto daemon-backed snapshots, pagination, and streams. It preserves the public run-reading surface while letting the daemon own concurrency, recovery, and storage details.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Public run readers and observability", "Runs", and "Transport Contract" instead of duplicating them here
- FOCUS ON "WHAT" — preserve the public API shape while changing the backing contract
- MINIMIZE CODE — keep one daemon-backed reader path instead of mixing filesystem and daemon modes
- TESTS REQUIRED — unit and integration coverage are mandatory for compatibility, replay, and watch behavior
</critical>

<requirements>
1. MUST replace direct reads from workspace-local run files with daemon-backed snapshot, list, replay, tail, and watch queries.
2. MUST preserve public run summary normalization for status, timestamps, ordering, and replay semantics.
3. MUST keep public error behavior stable for incompatible schema and partial-event edge cases where the package still exposes them.
4. MUST avoid direct `run.db` reads from public consumers; SQLite remains an internal daemon implementation detail.
5. SHOULD keep the public package ergonomic for callers that only know workspace root and run ID.
6. MUST prefer one daemon-backed reader implementation and MUST NOT preserve a long-term workspace-filesystem fallback path as a compatibility layer.
</requirements>

## Subtasks
- [ ] 13.1 Replace filesystem-backed open and list paths with daemon-backed client queries.
- [ ] 13.2 Adapt replay, tail, and watch behavior to snapshot, pagination, and SSE stream contracts.
- [ ] 13.3 Preserve run summary normalization and public edge-case behavior during the migration.
- [ ] 13.4 Remove layout assumptions that require `.compozy/runs` under the workspace root.
- [ ] 13.5 Add compatibility tests covering list, open, replay, tail, and watch against daemon-backed runs.

## Implementation Details
Implement the public-reader migration described in the TechSpec "Public run readers and observability", "Runs", and "Transport Contract" sections. This task should keep the exported package stable for callers while moving all operational storage and concurrency semantics behind daemon-owned APIs.

### AGH Reference Files
- `~/dev/compozy/agh/internal/api/core/sse.go` — reference for streaming semantics that replace local file tailing.
- `~/dev/compozy/agh/internal/observe/observer.go` — reference for list and snapshot query shapes over daemon-owned state.
- `~/dev/compozy/agh/internal/store/sessiondb/session_db.go` — reference for keeping per-run SQLite internal and out of the public read contract.

### Relevant Files
- `pkg/compozy/runs/run.go` — current `Open` and run-summary loading logic tied to workspace-local metadata files.
- `pkg/compozy/runs/summary.go` — summary normalization that must stay stable after the backend switch.
- `pkg/compozy/runs/replay.go` — replay behavior that should move to daemon-backed event pagination.
- `pkg/compozy/runs/tail.go` — tail behavior that must align with daemon stream and cursor semantics.
- `pkg/compozy/runs/watch.go` — live watch behavior that currently assumes local filesystem change observation.
- `pkg/compozy/runs/layout/layout.go` — workspace-local run layout assumptions that should stop driving public reads.

### Dependent Files
- `internal/cli/root.go` — CLI command surfaces that rely on the public run-reader package must stay compatible.
- `internal/cli/commands.go` — run-oriented CLI flows depend on the reader package staying stable.
- `internal/core/model/run_scope.go` — run identity and artifacts must remain consistent with the public summary model.
- `internal/core/run/journal/journal.go` — persisted event ordering must remain compatible with public replay and tail expectations.

### Related ADRs
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — requires public run reads to stop treating workspace-local files as the source of truth.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — defines the snapshot, list, and stream contract that replaces direct file access.

## Deliverables
- Daemon-backed implementation of the public `pkg/compozy/runs` reader package.
- Compatibility-preserving list, open, replay, tail, and watch behavior.
- Removal of public dependence on workspace-local run layout.
- Unit tests with 80%+ coverage for public summary and replay behavior **(REQUIRED)**
- Integration tests covering daemon-backed list/open/watch flows **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Public run summaries preserve status normalization and timestamp behavior after moving to daemon-backed data sources.
  - [ ] Replay and tail logic preserve expected cursor ordering and partial-event handling behavior.
  - [ ] Watch helpers translate daemon stream events into the same public event surface callers already expect.
  - [ ] Public readers surface a stable error when the daemon is unavailable instead of silently attempting filesystem fallback.
  - [ ] Cursor pagination across replay and tail boundaries preserves ordering through resume and terminal-run edges.
- Integration tests:
  - [ ] Opening a daemon-managed run by workspace root and run ID returns the expected summary without reading workspace-local run files.
  - [ ] Listing runs through the public package returns the same ordering and filtering behavior after the migration.
  - [ ] Public watch and tail flows continue working across reconnects against daemon-backed streams.
  - [ ] Public readers return the expected normalized status for completed, failed, cancelled, and crashed daemon-managed runs.
  - [ ] Replay and tail against a daemon-managed run preserve event order across snapshot pagination boundaries.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Public run readers no longer depend on workspace-local operational files
- Existing callers can keep using the package without learning daemon internals
- Replay, tail, and watch semantics remain stable after the backend migration
