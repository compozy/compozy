---
status: completed
title: Integrate extension bootstrap into start, fix-reviews, and exec commands
type: backend
complexity: medium
dependencies:
  - task_08
---

# Task 09: Integrate extension bootstrap into start, fix-reviews, and exec commands

## Overview
Wire the run-scope bootstrap from task 07 and the extension manager lifecycle from task 08 into the CLI command entry points. `compozy start` and `compozy fix-reviews` always enable executable extensions. `compozy exec` gains an explicit `--extensions` flag that is off by default so ad hoc prompts keep their current fast path unless the operator opts in. This task is the connective tissue that makes extensions actually run end-to-end during real Compozy invocations.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` Data Flow and Impact Analysis.
</critical>

<requirements>
- MUST update the `start` and `fix-reviews` command handlers to call `OpenRunScope` with `EnableExecutableExtensions = true`.
- MUST add a `--extensions` flag (default `false`) to the `exec` command that enables executable extensions only when the operator passes it.
- MUST propagate the run scope handle through the kernel handler chain so `plan.Prepare` and the run executor both see the same manager instance.
- MUST call `Manager.Start(ctx)` before planning begins and `Manager.Shutdown(ctx)` on run teardown via `RunScope.Close`.
- MUST ensure context cancellation (SIGINT/SIGTERM from the CLI) flows through to `RunScope.Close` via `defer`, matching the current signal handling behavior.
- MUST leave every other command (`fetch-reviews`, `tasks list`, `ext ...`, `setup`, `upgrade`, etc.) untouched with respect to executable extension spawning.
- MUST preserve existing test behavior for `start`, `fix-reviews`, and `exec` when no extensions are installed.
</requirements>

## Subtasks
- [x] 09.1 Add the `--extensions` flag to the `exec` command in `internal/cli/commands.go` or the exec command file and propagate it to the command state struct.
- [x] 09.2 Update the `start` command handler to request `EnableExecutableExtensions = true` when calling the kernel prepare path.
- [x] 09.3 Update the `fix-reviews` command handler to request `EnableExecutableExtensions = true` similarly.
- [x] 09.4 Update the `exec` command handler to request `EnableExecutableExtensions = cmdState.extensionsEnabled` so the default stays off.
- [x] 09.5 Ensure `runPrepared` and the kernel handler for each command use the returned `RunScope` and call `Close(ctx)` via `defer`.
- [x] 09.6 Write tests for each command path covering the enabled and disabled cases.

## Implementation Details
See TechSpec "System Architecture → Data Flow → Run startup" for the activation matrix and "Impact Analysis" row on `internal/cli/commands.go` for the flag addition.

Place changes under:
- `internal/cli/commands.go` — `--extensions` flag on the exec command
- `internal/cli/run.go` — `runPrepared` plumbing for the run scope
- `internal/core/kernel/handlers.go` — Hook the per-command `EnableExecutableExtensions` decision into `realOperations`
- `internal/cli/commands_test.go` (new or existing) — coverage for each flag path

Key invariants:
- `exec` without `--extensions` must spawn zero extension subprocesses and emit zero extension audit records.
- `start` and `fix-reviews` must always spawn enabled extensions even when `--extensions` is absent (the flag does not exist on those commands).
- Signal handling (SIGINT/SIGTERM) must still cancel the run context and trigger `RunScope.Close`.

### Relevant Files
- `internal/cli/root.go` — Cobra root command with subcommand registration.
- `internal/cli/commands.go` — Defines command state and flag parsing for `start`, `fix-reviews`, `exec`.
- `internal/cli/run.go` — `prepareAndRun`/`runPrepared` glue that calls the kernel dispatcher.
- `internal/core/kernel/handlers.go` — `realOperations.Prepare` and `Execute` that now accept the run scope.
- `internal/core/extension/runtime.go` — `OpenRunScope` from task 07.
- `internal/core/extension/manager.go` — `Manager.Start/Shutdown` from task 08.
- `_techspec.md` Data Flow and Impact Analysis sections.

### Dependent Files
- Tasks 10 and 11 — Hook insertion relies on the manager being active inside the run scope by the time `plan.Prepare` runs.
- Task 12 (CLI management) — Unrelated to runtime activation but shares the same CLI structure.

### Related ADRs
- [ADR-002: Per-Run Extension Lifetime](adrs/adr-002.md) — Per-command opt-in policy and lifetime rules.

## Deliverables
- `--extensions` flag on `compozy exec` with default `false`.
- `start` and `fix-reviews` always enable executable extensions via `OpenRunScope`.
- Unit tests with 80%+ coverage for flag parsing and kernel wiring **(REQUIRED)**
- Integration tests verifying extensions spawn only for the correct command/flag combinations **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `exec` without `--extensions` records `EnableExecutableExtensions = false` in the kernel options.
  - [x] `exec --extensions` records `EnableExecutableExtensions = true`.
  - [x] `start` records `EnableExecutableExtensions = true` regardless of any flag.
  - [x] `fix-reviews` records `EnableExecutableExtensions = true` regardless of any flag.
  - [x] `RunScope.Close` is called via `defer` on normal completion, cancellation, and error paths.
- Integration tests:
  - [x] `compozy exec` on a fixture workflow with a workspace extension installed does not spawn any extension subprocess.
  - [x] `compozy exec --extensions` on the same fixture spawns the extension, runs hooks, and records audit entries.
  - [x] `compozy start` on the same fixture spawns the extension regardless of the absence of `--extensions`.
  - [x] SIGINT during a run with extensions active cleanly terminates all extension subprocesses via `RunScope.Close`.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- `exec` without the flag retains its current fast path and produces zero extension artifacts
- `start` and `fix-reviews` always activate installed extensions
- Signal-based shutdown drains and kills extensions within the shutdown deadline
