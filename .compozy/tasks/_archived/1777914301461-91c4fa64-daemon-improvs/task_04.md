---
status: completed
title: Client and Run-Reader Contract Adoption
type: refactor
complexity: high
dependencies:
  - task_01
  - task_02
  - task_03
---

# Client and Run-Reader Contract Adoption

## Overview

This task moves daemon-facing clients onto the canonical contract after transport parity is in place. It replaces the blanket five-second timeout policy, aligns snapshot and event paging semantics, and updates `pkg/compozy/runs` to preserve public behavior through explicit adapters instead of ad hoc daemon payload knowledge.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Component Overview", "Timeout Policy", "Snapshot Integrity Semantics", and "Integration Points" instead of duplicating them here
- FOCUS ON "WHAT" - adopt the canonical contract and timeout rules without breaking public run-reader behavior silently
- MINIMIZE CODE - share decoding, timeout, and cursor logic instead of duplicating similar transport parsing in multiple client packages
- TESTS REQUIRED - client, stream, and run-reader compatibility tests are mandatory
</critical>

<requirements>
1. MUST migrate `internal/api/client` to the canonical contract package for request and response payloads, remote errors, event paging, and stream parsing.
2. MUST replace the blanket `5s` client timeout with the TechSpec timeout classes `probe`, `read`, `mutate`, `long_mutate`, and `stream`, including the heartbeat gap rule for reconnectable streams.
3. MUST update `pkg/compozy/runs` to consume canonical daemon payloads while preserving public reader behavior through explicit adapters where compatibility requires it.
4. MUST keep snapshot, paging, and stream cursor semantics aligned across internal clients, public readers, and CLI-facing consumers.
5. SHOULD avoid implicit breaking changes by making any remaining compatibility shims explicit and local to the client or run-reader boundary.
</requirements>

## Subtasks

- [x] 4.1 Rework `internal/api/client` to use canonical request, response, error, cursor, and timeout helpers.
- [x] 4.2 Implement operation-class timeouts and stream reconnect rules in the daemon client.
- [x] 4.3 Update `pkg/compozy/runs` to decode canonical snapshot, event-page, and stream payloads through explicit adapters.
- [x] 4.4 Align CLI-facing consumers of run and daemon client behavior with the updated timeout and payload semantics.
- [x] 4.5 Add regression coverage for snapshot compatibility, run watch behavior, and remote error handling.

## Implementation Details

Implement the consumer migration described in the TechSpec sections "Timeout Policy", "Snapshot Integrity Semantics", "Impact Analysis", and "Build Order". This task should update daemon-facing clients and run readers after transport parity is available, but it should not absorb the runtime hardening or observability implementation work that follows.

### Relevant Files

- `internal/api/client/client.go` - base daemon transport client needs canonical payload ownership and timeout-class routing.
- `internal/api/client/runs.go` - run snapshot, event paging, cancel, and stream logic must adopt canonical cursors and stream semantics.
- `internal/api/client/operator.go` - operator-facing daemon requests must move onto the canonical error and timeout helpers.
- `internal/api/client/reviews_exec.go` - review and exec flows must share the same canonical request and response handling.
- `pkg/compozy/runs/run.go` - public run-reader bootstrap currently owns daemon payload details that should move behind adapters.
- `pkg/compozy/runs/replay.go` - replay semantics depend on the canonical event page and cursor contract.
- `pkg/compozy/runs/remote_watch.go` - remote watch behavior must align with stream heartbeat and reconnect semantics.

### Dependent Files

- `pkg/compozy/runs/watch.go` - watch behavior depends on the canonical stream cursor and reconnect logic adopted here.
- `pkg/compozy/runs/integration_test.go` - public run-reader integration coverage must lock compatibility through the harness.
- `internal/cli/runs.go` - CLI run inspection commands consume the updated client and reader behavior.
- `internal/cli/reviews_exec_daemon.go` - review-fix daemon flows rely on the updated client request and response handling.
- `internal/cli/daemon.go` - daemon subcommands and health/status probes should inherit the new timeout classes.

### Related ADRs

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) - requires internal clients and public run readers to converge on one contract shape.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - requires parity and regression coverage for transport consumers.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) - snapshot and replay consumers depend on the stronger observability contract.

## Deliverables

- Canonical contract adoption across `internal/api/client` and `pkg/compozy/runs`.
- Operation-class timeout and stream reconnect behavior implemented in daemon-facing clients.
- Explicit adapter layer where public run-reader compatibility must be preserved.
- Unit tests with 80%+ coverage for timeout routing, remote error decoding, cursor handling, and snapshot adaptation **(REQUIRED)**
- Integration tests proving client and run-reader parity against a real daemon harness **(REQUIRED)**

## Tests

- Unit tests:
  - [x] A `GET /api/daemon/health` probe uses the `probe` timeout class while `POST /api/tasks/:slug/runs` uses `long_mutate` and `POST /api/runs/:run_id/cancel` uses `mutate`.
  - [x] A remote error envelope containing `conflict` or `schema_too_new` decodes to the expected client error with request ID preserved.
  - [x] `GetRunSnapshot` preserves current snapshot fields and only adds explicit compatibility handling where the canonical payload differs.
  - [x] Stream reconnect logic resumes from the last acknowledged cursor when no frame arrives within the configured heartbeat gap.
- Integration tests:
  - [x] `pkg/compozy/runs.Open` and internal client snapshot calls return consistent run metadata for the same harnessed daemon run.
  - [x] `pkg/compozy/runs` remote watch and daemon client stream consumption both survive a heartbeat-only idle period and resume correctly on subsequent events.
  - [x] CLI-facing run list or show commands still present stable output after the client contract migration.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Daemon clients use the TechSpec timeout classes instead of a blanket five-second timeout
- `pkg/compozy/runs` consumes canonical daemon payloads through explicit compatibility adapters
- Snapshot, paging, and stream cursor semantics stay aligned across client, reader, and CLI consumers
