---
status: completed
title: Runtime Shutdown, Logging, and Storage Discipline
type: infra
complexity: critical
dependencies:
    - task_02
    - task_03
---

# Runtime Shutdown, Logging, and Storage Discipline

## Overview

This task hardens daemon lifecycle behavior inside the current runtime boundary without redesigning the daemon core. It introduces bounded shutdown ownership, foreground versus detached signal discipline, structured daemon logging, and explicit SQLite checkpoint behavior for `global.db` and `run.db`.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Component Overview", "Technical Dependencies", "Monitoring and Observability", and "Build Order" instead of duplicating them here
- FOCUS ON "WHAT" - harden the existing daemon lifecycle boundary rather than introducing a new runtime architecture
- MINIMIZE CODE - centralize shutdown, logging, and close semantics instead of scattering new background control paths
- TESTS REQUIRED - lifecycle, logging, and storage close-path coverage are mandatory
</critical>

<requirements>
1. MUST replace unbounded `context.Background()` shutdown and close paths with bounded lifecycle ownership consistent with the TechSpec shutdown model.
2. MUST define explicit foreground versus detached daemon signal ownership so detached shutdown no longer depends on CLI-only contexts.
3. MUST add `internal/logger` for structured JSON daemon logging, file sink creation, stderr mirroring in foreground mode, and log rotation policy.
4. MUST add checkpoint discipline for `global.db` and `run.db` close paths so daemon shutdown does not rely on raw SQLite handle closure alone.
5. SHOULD preserve compile-safe Windows behavior even where Unix signal and process semantics remain richer in this phase.
</requirements>

## Subtasks

- [ ] 5.1 Introduce bounded host and transport shutdown contexts for daemon close paths.
- [ ] 5.2 Define signal ownership between CLI foreground launch and detached daemon lifecycle.
- [ ] 5.3 Add `internal/logger` with JSON file sink, rotation, and foreground stderr mirroring.
- [ ] 5.4 Add explicit checkpoint-on-close behavior for `global.db` and `run.db`.
- [ ] 5.5 Expand tests for graceful stop, forced stop, log sink startup policy, and database close discipline.

## Implementation Details

Implement the runtime hardening described in the TechSpec sections "Technical Dependencies", "Monitoring and Observability", "Impact Analysis", and "Build Order". This task should keep `Host`, `RunManager`, `global.db`, and `run.db` as the ownership model while tightening the shutdown, logging, and close-path behavior inside that boundary.

### Relevant Files

- `internal/daemon/host.go` - current daemon run and close flow uses unbounded shutdown paths that must become explicit and bounded.
- `internal/daemon/shutdown.go` - existing shutdown behavior is the natural place to centralize force and graceful stop semantics.
- `internal/store/globaldb/global_db.go` - global database close behavior needs checkpoint discipline before raw handle closure.
- `internal/store/rundb/run_db.go` - per-run database close behavior needs the same checkpoint discipline and bounded lifecycle semantics.
- `internal/logger/logger.go` - new structured daemon logger entrypoint and sink policy.
- `internal/cli/daemon_launch_unix.go` - foreground and detached daemon launch semantics on Unix depend on the signal ownership rules landed here.
- `internal/cli/daemon_launch_other.go` - non-Unix daemon launch path must stay compile-safe while adopting the same ownership model.

### Dependent Files

- `cmd/compozy/main.go` - binary startup should inherit the clarified foreground versus detached lifecycle ownership.
- `internal/daemon/boot_integration_test.go` - daemon start and stop integration coverage must assert bounded shutdown behavior.
- `internal/daemon/shutdown_test.go` - explicit stop and force-stop semantics should be regression-tested here.
- `internal/cli/daemon_commands_test.go` - CLI daemon commands must reflect the updated stop and launch semantics.
- `internal/daemon/service.go` - `POST /api/daemon/stop` behavior depends on the bounded shutdown implementation introduced here.

### Related ADRs

- [ADR-002: Incremental Runtime Supervision Hardening Inside the Existing Daemon Boundary](adrs/adr-002.md) - keeps lifecycle hardening inside the current daemon ownership model.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - requires deterministic shutdown behavior and real-daemon coverage.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) - logging and shutdown visibility are part of the operator contract.

## Deliverables

- Bounded daemon shutdown and close-path orchestration across host, transports, and DB handles.
- New `internal/logger` package with JSON daemon log sink, rotation, and foreground mirroring policy.
- Explicit checkpoint-on-close behavior for `global.db` and `run.db`.
- Unit tests with 80%+ coverage for shutdown helpers, logger configuration, and DB close semantics **(REQUIRED)**
- Integration tests proving bounded stop behavior and daemon log creation under real start/stop flows **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] Closing a daemon runtime uses bounded contexts for HTTP, UDS, DB, and host cleanup instead of raw background contexts.
  - [ ] Logger configuration creates the expected detached file sink policy and foreground stderr mirroring behavior.
  - [ ] `GlobalDB.Close()` and `RunDB.Close()` execute checkpoint discipline before final handle closure.
  - [ ] Detached daemon startup fails clearly when the configured log file sink cannot be opened.
- Integration tests:
  - [ ] `POST /api/daemon/stop` drains a daemon with no active runs within the configured shutdown bounds and leaves no stale transport artifacts.
  - [ ] Force-stop and graceful-stop flows produce the expected log file and lifecycle behavior under a harnessed daemon.
  - [ ] Foreground daemon launch mirrors logs to stderr while detached launch writes structured JSON to `~/.compozy/logs/daemon.log`.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Daemon shutdown no longer depends on unbounded background contexts
- Structured daemon logging and rotation are owned by a shared `internal/logger` package
- `global.db` and `run.db` close paths apply explicit checkpoint discipline before process exit
