---
status: completed
title: Wire tasks run-multiple CLI Command and Non-UI Modes
type: backend
complexity: high
dependencies:
  - task_01
  - task_02
  - task_03
---

# Task 4: Wire tasks run-multiple CLI Command and Non-UI Modes

## Overview

This task exposes the daemon-backed multi-run parent through `compozy tasks run-multiple`. It applies shared task-run flags and runtime overrides, handles `parallel` fallback messaging, and supports detach and stream modes before the tabbed TUI is layered on top.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The implementation MUST add `compozy tasks run-multiple [slugs]`.
- The command MUST accept one comma-separated positional slug list.
- The command MUST reject missing, empty, or duplicate slugs before contacting the daemon.
- The command MUST apply the same shared runtime flags that are valid for task runs.
- The command MUST read `[tasks.run] run_multiple_mode` and default to `enqueued`.
- If the configured mode is `parallel`, the command MUST print a clear V2/worktree fallback message and send enqueued execution.
- The command MUST preserve existing `tasks run` flags and behavior.
- Non-UI attach modes MUST provide useful parent run output without requiring the tabbed TUI.
</requirements>

## Subtasks

- [x] 4.1 Add a new Cobra subcommand under `tasks`.
- [x] 4.2 Add command state and defaults for task-run shared flags, runtime overrides, and config application.
- [x] 4.3 Resolve mode and print the `parallel` fallback message when needed.
- [x] 4.4 Start the daemon parent multi-run through the client method from task 02.
- [x] 4.5 Support detach and stream output for parent multi-runs.
- [x] 4.6 Add CLI tests for command registration, parser failures, fallback messaging, daemon request shape, and existing `tasks run` stability.

## Implementation Details

Keep the new command beside `newTasksRunCommandWithDefaults` rather than modifying the single-run command's argument contract. Use the parser and config behavior from task 01 and the client/API surface from task 02. See TechSpec "Component Overview" and "Known Risks" for command boundaries and fallback behavior.

### Relevant Files

- `internal/cli/daemon_commands.go` — current `tasks` command registration, single-run command, daemon client interface, and start handling.
- `internal/cli/root.go` — command kind constants and workflow execution classification.
- `internal/cli/state.go` — runtime config and executable extension enablement by command kind.
- `internal/cli/workspace_config.go` — project config defaults applied to command state.
- `internal/cli/run_observe.go` — current attach/watch helpers for daemon-backed runs.
- `internal/cli/commands_test.go` — command flag registration and default behavior tests.
- `internal/cli/daemon_commands_test.go` and `internal/cli/root_command_execution_test.go` — daemon-backed CLI tests.

### Dependent Files

- `internal/api/client/client.go` — must expose multi-run methods added in task 02.
- `internal/daemon/run_manager.go` — must implement parent behavior from task 03.
- `README.md` — later docs task will describe the final command examples.

### Related ADRs

- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md) — Requires a new command instead of overloading `tasks run`.
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md) — Defines `run-multiple`, `run_multiple_mode`, and fallback messaging.
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) — Requires the CLI to start a daemon parent run.

## Deliverables

- `compozy tasks run-multiple` command registered under `tasks`.
- Config-driven mode resolution and `parallel` fallback messaging.
- Detach and stream output for parent multi-runs.
- Unit tests with 80%+ coverage for command parsing and config behavior **(REQUIRED)**.
- CLI integration tests that verify daemon request shape and single-run stability **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] `tasks run-multiple alpha,beta --detach` sends ordered slugs `alpha`, `beta` to the multi-run client method.
  - [x] Missing slug input returns a user-facing workflow slug error.
  - [x] `alpha,,beta` returns an empty-slug validation error without contacting the daemon.
  - [x] `alpha,beta,alpha` returns a duplicate-slug validation error without contacting the daemon.
  - [x] Configured `parallel` prints a fallback message that mentions V2 and worktree isolation.
  - [x] `tasks run` still accepts exactly one slug and still calls `StartTaskRun`.
- Integration tests:
  - [x] In-process CLI daemon test starts a parent multi-run in detach mode and prints the parent run ID.
  - [x] Stream mode follows parent queue events and exits with a non-zero code on parent failure.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing.
- Test coverage >=80%.
- Users can start a daemon-owned multi-run parent from one command.
- Parallel fallback is explicit and deterministic.
- Existing single-run command output and attach behavior are unchanged.
