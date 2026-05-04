---
status: completed
title: Extension Runtime Daemon Adaptation
type: backend
complexity: high
dependencies:
  - task_04
  - task_05
---

# Extension Runtime Daemon Adaptation

## Overview
This task keeps Compozy's current executable extension model while adapting runtime ownership to daemon-managed runs. Extensions remain per-run subprocesses speaking stdio JSON-RPC, but they gain the daemon-aware host access and audit behavior needed once runs are no longer launched directly from the CLI process.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Integration Points", "Run manager", and "Transport Contract" instead of duplicating them here
- FOCUS ON "WHAT" — preserve the current extension model and evolve the ownership boundary around it
- MINIMIZE CODE — keep JSON-RPC on stdio and adapt daemon integration through narrow host-API seams
- TESTS REQUIRED — unit and integration coverage are mandatory for host access, hook auditing, and daemon-owned run execution
</critical>

<requirements>
1. MUST preserve executable extensions as run-scoped subprocesses and keep stdio JSON-RPC as the extension transport boundary.
2. MUST adapt extension runtime ownership so daemon-managed runs can initialize, audit, and shut down extensions correctly.
3. MUST support per-run Host API capability tokens for extension calls that need daemon-owned services.
4. MUST record JSON-RPC failures and hook outcomes in persisted run state without crashing unrelated runs.
5. SHOULD keep existing extension SDK contracts stable unless a daemon-specific capability must be added explicitly.
</requirements>

## Subtasks
- [x] 10.1 Adapt extension runtime startup and teardown to daemon-owned run lifecycle instead of direct CLI ownership.
- [x] 10.2 Introduce per-run Host API capability token handling for daemon callbacks.
- [x] 10.3 Preserve review-provider and hook behavior under daemon-managed execution.
- [x] 10.4 Persist extension audit outcomes and transport failures into run-owned storage.
- [x] 10.5 Add tests covering daemon-owned hook execution, host access, and extension shutdown behavior.

## Implementation Details
Implement the daemon-aware extension model described in the TechSpec "Integration Points", "Run manager", and "Transport Contract" sections. This task should preserve today's extension ergonomics and runtime contract while shifting ownership of initialization, auditing, and shutdown to the daemon-managed run lifecycle.

### AGH Reference Files
- `~/dev/compozy/agh/internal/session/manager.go` — reference for binding runtime-owned subprocesses and hooks to session lifecycle.
- `~/dev/compozy/agh/internal/daemon/daemon.go` — reference for coordinating subsystem ownership under daemon lifecycle.
- `~/dev/compozy/agh/internal/store/sessiondb/session_db.go` — reference for persisting per-run audit and lifecycle records.

### Relevant Files
- `internal/core/extension/runtime.go` — current run-scoped extension runtime that must move under daemon-owned run lifecycle.
- `internal/core/extension/host_api.go` — host API routing that must support daemon-managed capabilities.
- `internal/core/extension/capability.go` — capability semantics for host access that will need a per-run token model.
- `internal/core/extension/review_provider_bridge.go` — review-provider integration that must still work during daemon-backed review runs.
- `sdk/extension/host_api.go` — public SDK host API surface that should remain stable for extension authors.
- `internal/daemon/extension_bridge.go` — new daemon-facing bridge for extension host access and lifecycle wiring.

### Dependent Files
- `internal/core/run/executor/hooks.go` — executor hook dispatch must preserve extension behavior under daemon-managed runs.
- `internal/core/run/exec/hooks.go` — ad hoc exec hooks must follow the same daemon-owned extension lifecycle.
- `internal/store/rundb/run_db.go` — persisted hook audit rows and JSON-RPC failure data depend on the runtime outcomes defined here.
- `internal/api/core/handlers.go` — daemon-run creation and cancellation semantics must remain compatible with extension lifecycle behavior.

### Related ADRs
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — requires extension execution outcomes to live in operational run state.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — constrains how daemon-owned host access is surfaced to clients and runtimes.

## Deliverables
- Daemon-owned extension lifecycle integration for task, review, and exec runs.
- Per-run Host API capability token handling for extension callbacks.
- Persisted hook and JSON-RPC audit behavior aligned with run storage.
- Unit tests with 80%+ coverage for host capability and runtime adaptation **(REQUIRED)**
- Integration tests covering real daemon-backed extension execution and shutdown **(REQUIRED)**

## Tests
- Unit tests:
  - [x] A daemon-managed run initializes extensions with the expected run-scoped capability and audit context.
  - [x] Host API calls without a valid per-run capability token are rejected with explicit errors.
  - [x] Extension startup and shutdown preserve the existing hook ordering contract under daemon-owned runs.
  - [x] Extension JSON-RPC failures are recorded in persisted hook state without aborting unrelated runs.
  - [x] A failing extension hook can mark its own run outcome without corrupting other active runs or the daemon process.
- Integration tests:
  - [x] A daemon-backed task run executes installed extension hooks and records the outcomes in run-owned storage.
  - [x] A daemon-backed review run preserves review-provider bridge behavior through the adapted runtime.
  - [x] A daemon-backed exec run keeps the same extension and host API behavior as task and review runs.
  - [x] Forced run shutdown tears down extension subprocesses cleanly without leaking background processes.
  - [x] An extension hook failure is visible in audit storage and the live run stream without crashing the daemon.
- Test coverage target: >=80%
- All tests must pass

## Verification Evidence
- `go test ./internal/core/extension -count=1`
- `go test ./internal/core/run/executor -count=1`
- `go test ./internal/daemon -count=1`
- `go test -cover ./internal/core/extension` → `80.5%`
- `go test -cover ./internal/daemon` → `80.1%`
- `make verify`

## Success Criteria
- All tests passing
- Test coverage >=80%
- Extensions remain run-scoped subprocesses with stable JSON-RPC behavior
- Daemon-managed runs can initialize, audit, and shut down extensions predictably
- Host API access is explicit, per-run, and persisted through operational audit state
