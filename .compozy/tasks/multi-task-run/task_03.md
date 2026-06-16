---
status: completed
title: Add Parallel CLI Controls and Request Wiring
type: backend
complexity: medium
dependencies:
  - task_01
  - task_02
---

# Task 3: Add Parallel CLI Controls and Request Wiring

## Overview

This task adds the user-facing CLI switches for true parallel multi-run execution. It resolves mode and limit precedence, rejects invalid flag combinations early, and passes the resolved values to the daemon request.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- `tasks run --multiple ... --parallel` MUST resolve multi-run mode to `parallel`.
- `--parallel` MUST be valid only with `--multiple`.
- `--parallel-limit <n>` MUST be valid only with `--multiple`.
- `--parallel-limit <n>` MUST reject zero and negative values before daemon contact.
- `--parallel` MUST override configured `run_multiple_mode`.
- `--parallel-limit` MUST override configured `run_multiple_parallel_limit`.
- The current configured-parallel fallback message MUST be removed.
- Existing single-task `tasks run <slug>` behavior MUST remain unchanged.
</requirements>

## Subtasks

- [x] 3.1 Add CLI state and flags for `--parallel` and `--parallel-limit`.
- [x] 3.2 Resolve multi-run mode precedence from CLI, config, and default.
- [x] 3.3 Resolve parallel limit precedence from CLI, config, and default.
- [x] 3.4 Validate invalid flag combinations before daemon contact.
- [x] 3.5 Pass mode and parallel limit to `StartTaskRunMultiple`.
- [x] 3.6 Update command help and CLI tests.

## Implementation Details

Use the TechSpec "CLI And Configuration" section. This task should wire request data only; daemon acceptance and execution of parallel mode are handled in later scheduler tasks.

### Relevant Files

- `internal/cli/state.go` — command state for new flag values.
- `internal/cli/daemon_commands.go` — task run flags, multiple-run request construction, and current fallback logic.
- `internal/cli/daemon_commands_test.go` — mode resolution and multi-run stream tests.
- `internal/cli/root_command_execution_test.go` — end-to-end CLI command execution with in-process daemon helpers.
- `internal/cli/testdata/tasks_run_help.golden` — command help expectations.

### Dependent Files

- `internal/core/workspace/config_types.go` — supplies effective mode and limit helpers.
- `internal/api/core` — request type receives the resolved limit.
- `internal/daemon/task_multi.go` — later task validates and executes the requested mode.
- `README.md` — later task documents the new flags.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Defines `--parallel` as the explicit user opt-in.
- [ADR-008: Make the Parallel Multi-Run Limit Configurable](adrs/adr-008.md) — Defines CLI override behavior for the fanout limit.

## Deliverables

- `--parallel` and `--parallel-limit` flags on `tasks run`.
- Resolved mode and limit precedence implemented in CLI code.
- Invalid flag combinations rejected before daemon calls.
- CLI help updated.
- Unit tests with 80%+ coverage for mode/limit resolution helpers **(REQUIRED)**.
- Integration tests for daemon request wiring **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] `--parallel` resolves mode to `parallel` when config is unset.
  - [x] `--parallel` overrides config value `enqueued`.
  - [x] `--parallel-limit 3` overrides config/default limit.
  - [x] `--parallel-limit 0` returns a command error.
  - [x] `--parallel` without `--multiple` returns a command error.
  - [x] `--parallel-limit` without `--multiple` returns a command error.
- Integration tests:
  - [x] `tasks run --multiple alpha,beta --parallel` sends daemon mode `parallel`.
  - [x] `tasks run --multiple alpha,beta --parallel --parallel-limit 3` sends parallel limit `3`.
  - [x] `tasks run alpha` behavior remains unchanged.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Users can request true parallel mode and a per-invocation limit from the CLI.
- No fallback-to-enqueued message remains in CLI mode resolution.
