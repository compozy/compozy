---
status: completed
title: Extension manager lifecycle with spawn, initialize, shutdown, and health
type: backend
complexity: high
dependencies:
  - task_01
  - task_06
  - task_07
---

# Task 08: Extension manager lifecycle with spawn, initialize, shutdown, and health

## Overview
Implement the `extension.Manager` runtime lifecycle: spawn enabled subprocess extensions, run the initialize handshake, register their hook declarations with the dispatcher from task 05, wire the Host API router from task 06 as their callback target, run the optional health probe loop, and perform cooperative shutdown with SIGTERM/SIGKILL escalation. This task turns the discovery, dispatcher, and Host API pieces from earlier tasks into a living system that can actually talk to an extension subprocess.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md`, `_protocol.md`, and ADR-002/ADR-003.
</critical>

<requirements>
- MUST implement `Manager.Start(ctx)` that spawns every enabled subprocess extension via `internal/core/subprocess.Launch`, performs the initialize handshake per `_protocol.md` section 4, and transitions each extension to `Ready`.
- MUST pass `runtime.run_id`, `runtime.parent_run_id`, `runtime.workspace_root`, `runtime.invoking_command`, and the granted capability list in the initialize request.
- MUST inject `COMPOZY_RUN_ID`, `COMPOZY_PARENT_RUN_ID`, `COMPOZY_WORKSPACE_ROOT`, `COMPOZY_EXTENSION_NAME`, and `COMPOZY_EXTENSION_SOURCE` into each spawned subprocess's environment per `_protocol.md` section 1.6.
- MUST serve the `HostAPIRouter` from task 06 as the extension→host request handler for each subprocess's reader loop.
- MUST implement the optional `health_check` probe loop using the interval from `runtime.health_check_interval_ms`, marking an extension unhealthy per `_protocol.md` section 8.6.
- MUST implement `Manager.Shutdown(ctx)` that sends `shutdown` to every extension, waits for the deadline, and escalates SIGTERM → SIGKILL via the shared subprocess package.
- MUST publish `EventKindExtensionLoaded`, `EventKindExtensionReady`, and `EventKindExtensionFailed` on the event bus during lifecycle transitions.
- MUST not respawn crashed extensions within the same run (ADR-002).
- MUST forward `on_event` deliveries to extensions with `events.read` on a best-effort basis, honoring the bus's per-subscriber bounded queue semantics.
- MUST refuse to spawn an extension whose initialize handshake returns an unsupported protocol version or an inconsistent capability/hook contract.
</requirements>

## Subtasks
- [x] 08.1 Implement `Manager.Start(ctx)` spawning via `subprocess.Launch` with env injection and per-extension initialization workers.
- [x] 08.2 Implement the initialize handshake client: send request, validate response per `_protocol.md` section 4.4, reject version or capability mismatch.
- [x] 08.3 Wire each subprocess's reader loop to the `HostAPIRouter` so Host API calls are dispatched to service handlers.
- [x] 08.4 Implement the health probe loop with the interval/timeout from the initialize runtime block and the unhealthy threshold from `_protocol.md` section 8.6.
- [x] 08.5 Implement `Manager.Shutdown(ctx)` with cooperative shutdown, stdin close, SIGTERM, and SIGKILL escalation per `_protocol.md` section 9.4.
- [x] 08.6 Implement best-effort `on_event` forwarding from the event bus subscription to each subscribed extension.
- [x] 08.7 Emit `EventKindExtensionLoaded/Ready/Failed` during transitions and add the new event kinds to `pkg/compozy/events` with typed payloads.
- [x] 08.8 Write tests using a mock extension binary that the test harness builds and spawns in `t.TempDir()`.

## Implementation Details
See TechSpec "Implementation Design → Core Interfaces → Extension manager public surface", "System Architecture → Data Flow" for run startup and shutdown sequences, `_protocol.md` sections 3, 4, 5, 7, 8, 9 for the full wire contract, and ADR-002 and ADR-003 for lifecycle and transport rationale.

Place files under:
- `internal/core/extension/manager.go` — `Manager` type and public methods
- `internal/core/extension/manager_spawn.go` — spawn and initialize handshake
- `internal/core/extension/manager_health.go` — health probe loop
- `internal/core/extension/manager_shutdown.go` — cooperative shutdown and escalation
- `internal/core/extension/manager_events.go` — event bus subscription and on_event forwarding
- `pkg/compozy/events/kinds/extension.go` — new event payloads
- `internal/core/extension/manager_test.go`
- `internal/core/extension/testdata/mock_extension/` — Go source for a mock extension binary used in tests

This is the largest task in the feature. It integrates everything from tasks 01, 05, 06, and 07 and is the first point where real subprocess spawn and JSON-RPC exchange happen end to end. The mock extension binary is critical for testing — do not rely on real third-party extensions.

New event kinds to add:
- `EventKindExtensionLoaded` — emitted after discovery returns the extension.
- `EventKindExtensionReady` — emitted after initialize handshake succeeds.
- `EventKindExtensionFailed` — emitted on any lifecycle failure, including unhealthy transitions.

### Relevant Files
- `internal/core/subprocess/` — From task 01. Provides `Launch`, `Process`, signal escalation.
- `internal/core/extension/dispatcher.go` — From task 05. `Manager.Start` populates the dispatcher.
- `internal/core/extension/host_api.go` — From task 05. `Manager` owns the router instance.
- `internal/core/extension/host_writes.go`, `host_reads.go`, `host_helpers.go` — From task 06.
- `internal/core/extension/runtime.go` — From task 07. `Manager` is constructed inside `OpenRunScope`.
- `pkg/compozy/events/bus.go` — Bus source for event forwarding.
- `pkg/compozy/events/event.go` — Destination for new event kinds.
- `_protocol.md` sections 3, 4, 5, 7, 8, 9 — Wire contract.
- `adrs/adr-002.md`, `adrs/adr-003.md` — Lifetime and transport rationale.

### Dependent Files
- `internal/core/extension/runtime.go` — `OpenRunScope` wires the manager into the run scope.
- Task 09 — Calls `Manager.Start` and `Manager.Shutdown` from command entry points.
- Tasks 10, 11 — Call `Manager.DispatchMutable` and `DispatchObserver` from pipeline phases.

### Related ADRs
- [ADR-002: Per-Run Extension Lifetime](adrs/adr-002.md) — No respawn within a run; shutdown escalation rules.
- [ADR-003: JSON-RPC 2.0 over stdio with Shared internal/core/subprocess Package](adrs/adr-003.md) — Transport contract.
- [ADR-006: Host API Surface for Extension Callbacks](adrs/adr-006.md) — Router wiring during spawn.

## Deliverables
- `Manager` with full lifecycle: spawn, init, health, shutdown, event forwarding.
- New event kinds `EventKindExtensionLoaded/Ready/Failed` with typed payloads in `pkg/compozy/events/kinds/extension.go`.
- Mock extension binary source under `internal/core/extension/testdata/mock_extension/`.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests exercising real spawn, handshake, hook dispatch, and shutdown against the mock extension **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Initialize handshake client rejects an extension that returns an unsupported `protocol_version`.
  - [x] Initialize handshake client rejects an extension whose `accepted_capabilities` is not a subset of `granted_capabilities`.
  - [x] Initialize handshake client rejects an extension that accepts `events.read` but reports `supports.on_event = false`.
  - [x] Health probe marks an extension unhealthy after one explicit `healthy: false` response.
  - [x] Health probe marks an extension unhealthy after two consecutive timeout failures.
  - [x] `Manager.Shutdown` sends `shutdown` to every extension and waits for the deadline before escalating to SIGTERM.
  - [x] `Manager.Shutdown` escalates to SIGKILL when the process does not exit before the post-SIGTERM grace window.
  - [x] `EventKindExtensionReady` is published after a successful initialize.
  - [x] `EventKindExtensionFailed` is published when health marks an extension unhealthy.
  - [x] `on_event` forwarding drops events when the per-extension queue is full and records the drop.
- Integration tests:
  - [x] A mock extension binary spawns, completes initialize, receives one hook dispatch, answers, and shuts down cleanly within the deadline.
  - [x] A mock extension that ignores `shutdown` is killed by SIGTERM/SIGKILL within the expected escalation window.
  - [x] A mock extension that calls `host.tasks.list` receives a successful response routed through the router.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- A mock extension can complete its full lifecycle (spawn → init → hook dispatch → Host API call → shutdown) end to end.
- No extension can bypass the initialize handshake or capability negotiation.
- Shutdown always terminates the process, even when the extension misbehaves.

## Verification Evidence
- `go test ./internal/core/extension -coverprofile=/tmp/ext.cover.out` reported 80.4% statement coverage.
- `make verify` passed end to end on 2026-04-10, including fmt, lint, tests, and build.
