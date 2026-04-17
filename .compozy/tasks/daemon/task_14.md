---
status: pending
title: Workspace, Daemon, Sync, and Archive Command Completion
type: refactor
complexity: high
dependencies:
  - task_06
  - task_09
  - task_11
---

# Workspace, Daemon, Sync, and Archive Command Completion

## Overview
This task completes the explicit operator command surface around the daemon and workspace registry. It turns the command skeleton into the user-facing control plane for daemon status, workspace registration, sync, and archive operations while preserving scriptable output and exit-code behavior.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "CLI/TUI clients", "Workspaces", "Task workflows", and "Transport Contract" instead of duplicating them here
- FOCUS ON "WHAT" — this task is about completing the command surface, not inventing new operational concepts
- MINIMIZE CODE — reuse the daemon client foundation and keep output rules explicit and testable
- TESTS REQUIRED — unit and integration coverage are mandatory for help text, output, and command behavior
</critical>

<requirements>
1. MUST complete the explicit `daemon`, `workspaces`, `sync`, and `archive` command families over daemon-backed APIs.
2. MUST keep `sync` and `archive` behavior governed by daemon state instead of direct filesystem heuristics.
3. MUST preserve script-friendly output formats and stable exit behavior for operator automation.
4. MUST expose workspace register, unregister, resolve, list, and show behavior consistent with the daemon registry contract.
5. SHOULD keep help text and flags aligned with the approved daemon command model from the TechSpec.
</requirements>

## Subtasks
- [ ] 14.1 Complete daemon status and stop commands over the shared daemon-client transport.
- [ ] 14.2 Complete workspace register, unregister, resolve, list, and show command behavior.
- [ ] 14.3 Finish daemon-backed sync and archive command execution paths.
- [ ] 14.4 Align CLI help text, flags, and output formatting with the new command surface.
- [ ] 14.5 Add tests covering command execution, output, and exit behavior for all completed command families.

## Implementation Details
Implement the operator surface described in the TechSpec "CLI/TUI clients", "Workspaces", "Task workflows", and "Transport Contract" sections. This task should make the daemon-backed operational commands feel complete and scriptable without reintroducing hidden filesystem heuristics behind the scenes.

### Relevant Files
- `internal/cli/root.go` — root command registration for the explicit daemon and workspace families.
- `internal/cli/commands.go` — command construction and output wiring for the new daemon-backed surface.
- `internal/cli/workspace_config.go` — workspace targeting and config resolution used by sync and archive commands.
- `internal/cli/validate_tasks.go` — validation behavior that must remain compatible with the daemon-backed task surface.
- `internal/core/migration/migrate.go` — legacy migration hooks that may still be invoked during archive and sync cleanup flows.
- `internal/core/tasks/store.go` — task artifact expectations that sync and archive commands must preserve.

### Dependent Files
- `internal/cli/root_command_execution_test.go` — top-level command behavior and output tests must cover the completed command families.
- `internal/cli/workspace_config_test.go` — workspace resolution tests depend on the command semantics introduced here.
- `internal/cli/validate_tasks_test.go` — validation command behavior must remain stable alongside the new command surface.
- `internal/cli/commands_test.go` — help text, flag wiring, and command registration tests depend on this task.

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — defines the explicit daemon lifecycle surface.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — constrains the command/API alignment.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — requires explicit workspace commands and UX continuity.

## Deliverables
- Completed daemon and workspace command families over daemon-backed APIs.
- Daemon-backed sync and archive command implementations with stable output behavior.
- Updated help text and flag wiring for the approved command surface.
- Unit tests with 80%+ coverage for command parsing and output rules **(REQUIRED)**
- Integration tests covering daemon, workspace, sync, and archive command execution **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Help text and flags for daemon and workspace commands match the approved command surface and do not expose the removed `start` path.
  - [ ] Command output formatting and exit behavior remain stable for success, validation, and conflict cases.
  - [ ] Workspace targeting resolves explicit path, current workspace, and daemon-registry lookups consistently.
- Integration tests:
  - [ ] `compozy daemon status` and `compozy daemon stop` operate correctly against a real daemon instance.
  - [ ] `compozy workspaces register|show|list|unregister` reflects the daemon registry accurately and rejects unregister with active runs.
  - [ ] `compozy sync` and `compozy archive` operate through daemon-backed state rather than direct metadata-file heuristics.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon and workspace command families are complete and scriptable
- Sync and archive commands operate on daemon-backed state consistently
- Help text and exit behavior align with the approved CLI model
