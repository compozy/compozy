---
status: completed
title: Early run-scope bootstrap kernel refactor
type: refactor
complexity: high
dependencies:
  - task_03
  - task_04
  - task_05
---

# Task 07: Early run-scope bootstrap kernel refactor

## Overview
Refactor `internal/core/kernel` so that run artifacts, the run journal, the event bus, and (for extension-aware commands) the extension manager are allocated **before** `plan.Prepare()` runs. This is the structural enabler for `plan.*` and `prompt.*` hooks in v1: without this refactor, the extension manager would only exist after planning completes, which is too late for those hook families to participate.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` Core Interfaces and Data Flow sections.
</critical>

<requirements>
- MUST introduce an `OpenRunScope` function (or method on the operations interface) that allocates run artifacts, opens the journal, constructs the event bus, and optionally constructs the extension manager before planning runs.
- MUST produce a `RunScope` value carrying the artifacts, journal, bus, `ExtensionsEnabled` flag, and `Manager` handle per the TechSpec "Core Interfaces → Run-scope bootstrap" section.
- MUST add `OpenRunScopeOptions{EnableExecutableExtensions bool}` so callers can opt in or out of executable extensions without changing the discovery/overlay path.
- MUST preserve all current behavior of `internal/core/kernel/handlers.go` `realOperations` for commands that do not enable executable extensions.
- MUST pass the `RunScope` into `plan.Prepare` so plan-phase hooks can run against the already-initialized manager.
- MUST ensure the `Close(ctx)` method on `RunScope` tears down the manager (when present), flushes the journal, and closes the bus in the correct order.
- MUST keep existing `runStartHandler` tests passing with updated wiring.
- MUST NOT spawn subprocesses, register hook insertion points, or touch CLI command surfaces in this task.
</requirements>

## Subtasks
- [x] 07.1 Define `RunScope` and `OpenRunScopeOptions` types in a new file under `internal/core/extension` (or `internal/core/kernel` if placed on the kernel side) per TechSpec Core Interfaces guidance.
- [x] 07.2 Implement `OpenRunScope(ctx, cfg, opts)` that allocates run artifacts, opens the journal with `bus`, and (when `opts.EnableExecutableExtensions`) constructs the extension manager from discovery, capability, and dispatcher components.
- [x] 07.3 Update `internal/core/kernel/handlers.go` `realOperations.Prepare` to accept the `RunScope` and pass the manager into `plan.Prepare`.
- [x] 07.4 Update `plan.Prepare` signature to accept the manager (nilable). When nil, plan behaves exactly as today.
- [x] 07.5 Implement `RunScope.Close(ctx)` with ordered teardown: manager shutdown → journal close → bus close.
- [x] 07.6 Write tests covering the three-way disabled/enabled/nil-manager modes and the teardown ordering under context cancellation.

## Implementation Details
See TechSpec "Implementation Design → Core Interfaces → Run-scope bootstrap" for the `RunScope` and `OpenRunScopeOptions` shape, "System Architecture → Data Flow → Run startup" for the startup sequence, and "Impact Analysis" for the list of affected kernel/plan files.

Place files under:
- `internal/core/extension/runtime.go` — `RunScope`, `OpenRunScopeOptions`, `OpenRunScope`, `Close`
- `internal/core/kernel/handlers.go` — modified `realOperations.Prepare`
- `internal/core/plan/prepare.go` — modified signature to accept manager
- `internal/core/extension/runtime_test.go`

This task inevitably changes function signatures in `plan.Prepare` and `realOperations.Prepare`. Downstream callers include `internal/cli/run.go` and test files. The refactor is contained but spans multiple packages, which is why it rates high complexity.

Key invariants:
- Commands that do not enable executable extensions must see zero behavioral change.
- The extension manager is `nil` on the `RunScope` when `EnableExecutableExtensions = false`.
- `OpenRunScope` is the single entry point for allocating run artifacts plus journal plus bus — no duplicate allocation paths elsewhere.

### Relevant Files
- `internal/core/kernel/handlers.go` — `realOperations.Prepare` at the current run-start insertion point.
- `internal/core/kernel/dispatcher.go` — Type-safe dispatcher used by kernel handlers.
- `internal/core/plan/prepare.go` — Current `Prepare` signature that accepts `cfg` and `bus`.
- `internal/core/run/journal/journal.go` — Journal open/close lifecycle.
- `pkg/compozy/events/bus.go` — Bus construction and Close semantics.
- `_techspec.md` Core Interfaces and Data Flow sections.

### Dependent Files
- `internal/cli/run.go` — Calls `runPrepared` which calls `realOperations.Prepare`. Will need to propagate the new `opts`.
- `internal/core/run/executor/execution.go` — Consumes the prepared run; will need the manager handle in a later task.
- Task 08 (manager lifecycle) — Depends on `OpenRunScope` to construct the manager it will then start.
- Task 09 (command integration) — Sets `opts.EnableExecutableExtensions` based on the invoking command.

### Related ADRs
- [ADR-002: Per-Run Extension Lifetime](adrs/adr-002.md) — The lifetime contract that makes per-run scope the right refactor.

## Deliverables
- `RunScope` and `OpenRunScope` implemented and integrated into `realOperations.Prepare`.
- `plan.Prepare` signature updated to accept a nullable manager without behavior change for existing callers.
- Updated kernel tests demonstrating both disabled and enabled extension modes.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests covering run-start with and without extensions enabled **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `OpenRunScope` with `EnableExecutableExtensions = false` returns a scope with artifacts, journal, bus, and `Manager == nil`.
  - [x] `OpenRunScope` with `EnableExecutableExtensions = true` and zero enabled extensions returns a scope with a non-nil empty manager.
  - [x] `OpenRunScope` with discovery returning enabled extensions returns a scope whose manager has those extensions registered but not yet started.
  - [x] `RunScope.Close` shuts down the manager (if present), then closes the journal, then closes the bus, in that order.
  - [x] `RunScope.Close` is safe to call when the manager is nil.
  - [x] `RunScope.Close` returns context deadline exceeded when shutdown exceeds the deadline and escalates cleanup.
  - [x] Updated `plan.Prepare` with `manager == nil` produces exactly the same output as the current implementation on a fixture workflow.
- Integration tests:
  - [x] `runStartHandler.Handle` on a fixture workflow with `EnableExecutableExtensions = false` still produces the same run artifacts as before this task.
  - [x] `runStartHandler.Handle` on a fixture workflow with `EnableExecutableExtensions = true` produces the same run artifacts plus a run-scoped audit log file.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- All existing kernel tests pass without behavioral regression
- The run scope is allocated exactly once per run, before `plan.Prepare` is called, for every extension-aware command
