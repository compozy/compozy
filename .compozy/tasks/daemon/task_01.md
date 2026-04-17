---
status: pending
title: Home-Scoped Daemon Bootstrap
type: infra
complexity: high
dependencies: []
---

# Home-Scoped Daemon Bootstrap

## Overview

This task establishes the daemon host as a `$HOME`-scoped singleton instead of a workspace-scoped process. It creates the reusable path resolution, layout creation, lock/info handling, and boot lifecycle needed before any transport or persistence work can rely on a stable daemon runtime.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Component Overview", "Stable Home Layout", and "Development Sequencing" instead of duplicating them here
- FOCUS ON "WHAT" — preserve existing Compozy behavior where possible and introduce the daemon host as a narrow seam
- MINIMIZE CODE — prefer adapting current workspace/config resolution over creating parallel path logic
- TESTS REQUIRED — unit and integration coverage are mandatory for all daemon bootstrap behavior
</critical>

<requirements>
1. MUST resolve daemon home paths from `$HOME` and never from the current workspace root or process `cwd`.
2. MUST create and validate the stable `~/.compozy` layout, including daemon directory ownership and required runtime files.
3. MUST implement singleton boot semantics with stale lock, stale socket, and stale daemon info cleanup before the daemon reports ready.
4. MUST keep daemon startup idempotent and safe when another healthy daemon is already running.
5. SHOULD mirror AGH's home-layout and boot-ordering patterns where they fit without importing AGH-only domains.
</requirements>

## Subtasks

- [ ] 1.1 Define the reusable home path and layout contract for the daemon runtime.
- [ ] 1.2 Add daemon bootstrap code that acquires singleton ownership and cleans stale runtime artifacts safely.
- [ ] 1.3 Introduce daemon status/info state so later clients can probe readiness without guessing from the filesystem.
- [ ] 1.4 Route daemon startup through the CLI/bootstrap layer without tying the daemon base path to workspace discovery.
- [ ] 1.5 Add tests covering idempotent start, stale artifact cleanup, and readiness gating.

## Implementation Details

Establish the host-runtime foundation described in the TechSpec "Component Overview", "Stable Home Layout", and "Development Sequencing" sections. This task should create the path/bootstrap layer that every later task depends on; it should not yet implement transports, storage schemas, or run lifecycle orchestration beyond the minimal readiness contract.

### AGH Reference Files

- `~/dev/compozy/agh/internal/daemon/boot.go` — reference for staged bootstrap, lock handling, stale-artifact cleanup, and readiness ordering.
- `~/dev/compozy/agh/internal/daemon/daemon.go` — reference for daemon lifecycle ownership and graceful shutdown boundaries.

### Relevant Files

- `cmd/compozy/main.go` — current binary entrypoint and natural place to wire daemon-aware startup.
- `compozy.go` — public embedding surface that currently assumes direct CLI execution.
- `internal/cli/root.go` — current root command registration and best-effort workspace boot path.
- `internal/core/workspace/config.go` — existing workspace/config resolution that currently starts from `cwd`.
- `internal/core/model/workspace_paths.go` — current workspace-scoped path helpers that must stop owning daemon home layout.
- `internal/config/home.go` — new home path resolution layer modeled after AGH's proven layout handling.
- `internal/daemon/boot.go` — new daemon bootstrap flow for lock, info, readiness, and stale-artifact handling.

### Dependent Files

- `internal/api/udsapi/server.go` — will need the socket path and daemon directory contract from this task.
- `internal/api/httpapi/server.go` — will depend on the daemon readiness and info-file semantics introduced here.
- `internal/store/globaldb/global_db.go` — will rely on the home layout and DB directory contract created here.
- `internal/cli/state.go` — later daemon client work will depend on the startup and readiness behavior defined here.

### Related ADRs

- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — establishes the singleton daemon posture and `$HOME` base.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — requires auto-start without visible UX regression.

## Deliverables

- Reusable home path resolution and layout creation for `~/.compozy`.
- Singleton daemon bootstrap with lock/info/socket cleanup and explicit readiness state.
- CLI/bootstrap integration for idempotent daemon start behavior.
- Unit tests with 80%+ coverage for new bootstrap code **(REQUIRED)**
- Integration tests covering idempotent start and stale-artifact recovery **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] Resolving home paths from `$HOME` returns the expected `daemon`, `db`, `runs`, `logs`, and `cache` locations regardless of current `cwd`.
  - [ ] Bootstrap returns a clear error when `$HOME` is missing, unusable, or cannot host the daemon directory.
  - [ ] A stale lock, stale socket, and stale info file are removed together when no healthy daemon owns them.
  - [ ] A healthy running daemon causes startup to reuse the existing singleton without taking over ownership.
  - [ ] Mismatched daemon runtime artifacts are rebuilt atomically so the info file, lock state, and socket path converge to one consistent daemon identity.
- Integration tests:
  - [ ] Starting the daemon twice from the same machine leaves exactly one healthy singleton instance and one readiness record.
  - [ ] Bootstrapping from a workspace subdirectory still uses the same home-scoped daemon layout and readiness contract.
  - [ ] Restarting after a killed daemon that left stale lock and socket artifacts recovers cleanly into one healthy singleton.
  - [ ] Invoking bootstrap from two unrelated workspaces on the same machine resolves the same `~/.compozy/daemon` ownership and does not create workspace-scoped copies.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- The daemon resolves and creates its runtime layout entirely under `$HOME`
- Bootstrap is idempotent and does not create duplicate daemons
- Stale singleton artifacts are cleaned safely before readiness is reported
