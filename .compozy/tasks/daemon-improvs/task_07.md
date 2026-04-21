---
status: pending
title: Observability, Snapshot Integrity, and Transcript Assembly
type: backend
complexity: critical
dependencies:
  - task_03
  - task_04
  - task_05
  - task_06
---

# Observability, Snapshot Integrity, and Transcript Assembly

## Overview

This task completes the operator-facing side of the daemon hardening effort by making observability a first-class contract. It enriches health and metrics, persists snapshot integrity state, exposes sticky `Incomplete` semantics, and assembles a stable transcript view from persisted run data for clients, readers, and CLI consumers.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Snapshot Integrity Semantics", "Monitoring and Observability", "API Endpoints", and "Key Decisions" instead of duplicating them here
- FOCUS ON "WHAT" - strengthen daemon and run visibility through canonical contract surfaces backed by durable state
- MINIMIZE CODE - reuse persisted projections and contract types instead of rebuilding observability data from ad hoc logs or in-memory UI state
- TESTS REQUIRED - snapshot integrity, metrics, and transcript replay coverage are mandatory
</critical>

<requirements>
1. MUST enrich `/api/daemon/health` and `/api/daemon/metrics` to expose the readiness, degraded reasons, reconcile warnings, counters, and schema specified by the TechSpec.
2. MUST persist durable run integrity state and expose sticky `RunSnapshot.Incomplete` semantics with reason-aware behavior rather than recalculating everything from logs on every request.
3. MUST assemble canonical transcript output from persisted projections so cold clients and run readers can inspect completed runs without relying on live in-memory state.
4. MUST keep snapshot, transcript, and observability semantics aligned across transports, daemon clients, public run readers, and CLI-facing consumers.
5. SHOULD bound snapshot and transcript payloads explicitly so richer observability does not create unbounded response growth.
</requirements>

## Subtasks

- [ ] 7.1 Expand daemon health and metrics surfaces to match the richer operational contract from the TechSpec.
- [ ] 7.2 Persist run integrity state and reason codes needed for sticky `Incomplete` snapshot semantics.
- [ ] 7.3 Upgrade snapshot assembly to surface integrity state, shutdown details, and canonical cursor behavior from durable projections.
- [ ] 7.4 Assemble canonical transcript output from persisted transcript and event projections for cold readers.
- [ ] 7.5 Add parity and integration coverage for health, metrics, snapshot integrity, and transcript replay behavior.

## Implementation Details

Implement the observability work described in the TechSpec sections "Snapshot Integrity Semantics", "Monitoring and Observability", "API Endpoints", and "Build Order". This task should build on the already-migrated contract, client, and runtime hardening layers rather than redefining them.

### Relevant Files

- `internal/daemon/service.go` - daemon health and metrics surfaces need to expose richer readiness, degraded reasons, and counter output.
- `internal/daemon/run_snapshot.go` - snapshot assembly must surface durable integrity state, cursor behavior, and transcript-backed read models.
- `internal/daemon/transport_mappers.go` - transport mappers must align service output with the canonical observability contract.
- `internal/store/rundb/run_db.go` - run persistence must store and load integrity state and transcript-related projections.
- `internal/core/run/transcript/model.go` - canonical transcript model behavior depends on the persisted data assembled here.
- `internal/core/run/transcript/render.go` - transcript rendering for cold replay should align with the canonical snapshot and transcript contract.
- `internal/api/client/runs.go` - client snapshot and event readers must consume the stronger observability contract consistently.

### Dependent Files

- `internal/store/rundb/migrations.go` - schema changes are required for durable integrity state and any new projection rows.
- `pkg/compozy/runs/replay.go` - public replay flows depend on canonical transcript and snapshot integrity semantics.
- `pkg/compozy/runs/watch.go` - watch and attach behavior must understand sticky `Incomplete` snapshot semantics.
- `internal/api/httpapi/transport_integration_test.go` - richer health, metrics, and snapshot behavior needs parity coverage here.
- `internal/daemon/service_test.go` - daemon observability behavior should be regression-tested at the service layer.

### Related ADRs

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) - observability payloads are part of the same canonical contract surface.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - requires parity and integration validation for these operator-facing surfaces.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) - defines health, metrics, snapshots, and transcript assembly as primary-scope work.

## Deliverables

- Richer daemon health and metrics output aligned with the TechSpec metric schema and degraded-state model.
- Durable run integrity state and sticky `RunSnapshot.Incomplete` behavior.
- Canonical transcript assembly for cold replay and inspection surfaces.
- Unit tests with 80%+ coverage for health details, metrics rendering, integrity state handling, and transcript assembly **(REQUIRED)**
- Integration tests proving health, metrics, snapshot, and transcript behavior across a real daemon harness **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] `GET /api/daemon/health` returns structured degraded details when reconcile warnings or integrity issues exist, and returns ready state when they do not.
  - [ ] Metrics rendering includes the required counters and gauges such as `daemon_active_runs`, `daemon_shutdown_conflicts_total`, and `daemon_acp_stall_total` with the expected labels.
  - [ ] Snapshot assembly marks `Incomplete` when journal drops, event gaps, or unrecoverable transcript gaps are persisted for a run and keeps the flag sticky on later reads.
  - [ ] Transcript assembly from persisted projections returns messages in deterministic order with bounded payload behavior.
- Integration tests:
  - [ ] A harnessed daemon exposes richer health and metrics output over both HTTP and UDS after startup and after a degraded runtime scenario.
  - [ ] A run with persisted integrity issues surfaces `Incomplete` and reason-aware snapshot behavior consistently through daemon client and public run-reader access.
  - [ ] Replaying a completed run after daemon restart reconstructs a stable transcript view from persisted data rather than relying on live in-memory state.
  - [ ] CLI-facing run inspection commands reflect the stronger snapshot and transcript behavior without diverging from transport responses.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Daemon health and metrics expose the richer operational contract defined by the TechSpec
- Snapshot integrity state is durable, sticky, and visible to transports and readers
- Transcript replay becomes a canonical persisted read model for cold inspection and post-run analysis
