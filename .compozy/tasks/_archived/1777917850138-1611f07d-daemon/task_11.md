---
status: completed
title: CLI Daemon Client Foundation
type: refactor
complexity: high
dependencies:
  - task_04
  - task_05
---

# CLI Daemon Client Foundation

## Overview
This task turns the CLI into a daemon client instead of a direct execution entrypoint. It introduces the shared daemon bootstrap and handshake path, resolves workspace and presentation mode before requests are sent, and lays the command foundation for the new daemon-oriented CLI surface.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "CLI/TUI clients", "Transport Contract", and "Development Sequencing" instead of duplicating them here
- FOCUS ON "WHAT" — the CLI should become a thin, predictable client over daemon contracts
- MINIMIZE CODE — reuse existing command parsing and config precedence instead of duplicating command trees
- TESTS REQUIRED — unit and integration coverage are mandatory for bootstrap, config resolution, and transport errors
</critical>

<requirements>
1. MUST add an `ensureDaemon()` client path before commands that now depend on daemon-backed execution or state.
2. MUST resolve workspace root, daemon transport target, and `auto -> ui|stream|detach` presentation mode on the client side before sending requests.
3. MUST preserve existing config precedence across home config, workspace config, and CLI flags.
4. MUST remove the legacy `compozy start` entrypoint and route interactive execution through the new daemon-backed command families.
5. MUST turn connection, handshake, and transport failures into stable CLI errors with readable output and deterministic exit behavior.
6. MUST NOT introduce deprecated CLI aliases or keep a second direct-execution path unless a later ADR explicitly requires one.
</requirements>

## Subtasks
- [x] 11.1 Introduce shared daemon bootstrap and client-handshake code for daemon-backed commands.
- [x] 11.2 Resolve workspace and presentation mode before request dispatch and keep config precedence explicit.
- [x] 11.3 Rework the command foundation to support daemon, workspaces, tasks, reviews, runs, sync, archive, and exec flows.
- [x] 11.4 Remove the legacy `start` command path and update CLI wiring to the new client model.
- [x] 11.5 Add tests covering bootstrap reuse, presentation-mode resolution, and transport failure behavior.

## Implementation Details
Implement the client-side control plane described in the TechSpec "CLI/TUI clients", "Transport Contract", and "Development Sequencing" sections. This task should keep the CLI thin and deterministic: configuration, daemon bootstrap, workspace resolution, and mode selection happen in the client, while actual operational state lives behind the daemon API.

### AGH Reference Files
- `~/dev/compozy/agh/internal/daemon/boot.go` — reference for `ensureDaemon()`-style client bootstrap and readiness probing.
- `~/dev/compozy/agh/internal/api/udsapi/server.go` — reference for the UDS transport assumptions the CLI is targeting.
- `~/dev/compozy/agh/internal/api/httpapi/server.go` — reference for localhost HTTP fallback and parity expectations.

### Relevant Files
- `internal/cli/root.go` — root command wiring that currently assumes direct execution paths.
- `internal/cli/commands.go` — command-family registration that must grow into the daemon-oriented surface.
- `internal/cli/state.go` — current command state and runtime wiring that should become daemon-client aware.
- `internal/cli/run.go` — current run-oriented command execution path that must stop owning run lifecycle.
- `internal/cli/workspace_config.go` — config precedence and workspace discovery that must stay correct under daemon bootstrap.
- `internal/cli/daemon_commands.go` — new daemon-client command bootstrap and shared request dispatch helpers.

### Dependent Files
- `internal/core/kernel/commands/run_start.go` — typed command adapters must align with the new client-side request model.
- `internal/core/kernel/handlers.go` — kernel entrypoints will be affected as daemon-backed commands replace direct local execution.
- `internal/cli/root_command_execution_test.go` — execution-oriented CLI tests must be updated to reflect the daemon bootstrap path.
- `internal/cli/workspace_config_test.go` — config precedence and interactive/default mode tests depend on the client semantics defined here.

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — requires auto-start and singleton-aware client bootstrap.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — defines the transport contract the CLI must consume.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — constrains presentation-mode defaults and UX continuity.

## Deliverables
- Shared daemon bootstrap and client handshake for daemon-backed CLI commands.
- Client-side workspace and presentation-mode resolution aligned with config precedence.
- CLI command foundation updated for the new daemon-oriented surface.
- Unit tests with 80%+ coverage for config and client bootstrap behavior **(REQUIRED)**
- Integration tests covering daemon reuse, transport failures, and legacy start removal **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Config precedence across home config, workspace config, and CLI flags resolves the expected explicit presentation mode before request dispatch.
  - [x] `auto` presentation mode resolves to `ui` on interactive TTYs and to `stream` on non-interactive invocations before the request is sent.
  - [x] `ensureDaemon()` reuses an already healthy daemon instead of starting a duplicate instance.
  - [x] A stale socket or stale daemon info file causes bootstrap to repair the transport path instead of returning a misleading connection error.
  - [x] Connection and handshake failures return stable CLI errors with the expected exit behavior.
- Integration tests:
  - [x] Running a daemon-backed tasks command from a workspace subdirectory resolves the correct workspace and transport target before issuing the request.
  - [x] Running the same daemon-backed command from a non-interactive environment resolves `auto` to the expected non-TUI mode.
  - [x] The legacy `compozy start` path is removed and the replacement daemon-backed command help reflects the new surface.
  - [x] A daemon-backed command can bootstrap the daemon automatically and continue execution without a second manual step.
  - [x] A command against an unhealthy daemon reports the daemon problem explicitly instead of falling back to silent local execution.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The CLI becomes a consistent daemon client instead of a direct executor
- Presentation mode and workspace targeting are resolved explicitly on the client
- Legacy start semantics are replaced without degrading operator UX
