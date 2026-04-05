---
status: pending
title: CLI kernel bootstrap, command refactor, and documentation
type: refactor
complexity: high
dependencies:
  - task_04
  - task_05
---

# Task 08: CLI kernel bootstrap, command refactor, and documentation

## Overview
Wire the Service Kernel into the Cobra CLI: construct `KernelDeps` at root startup, call `kernel.BuildDefault`, and update each of the nine command constructors to capture the dispatcher in their `runWorkflow` closures. Each command translates its flags into the typed command struct and calls `kernel.Dispatch`. Then retire the transitional `core.Config` public exports, and produce the two required documentation files (`docs/events.md`, `docs/reader-library.md`).

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST construct `KernelDeps{Logger, EventBus, Workspace, AgentRegistry}` once at root command initialization
- MUST call `kernel.BuildDefault(deps)` and pass the returned `*kernel.Dispatcher` into each command constructor
- MUST PRESERVE `commandState.runWorkflow func(context.Context, core.Config) error` signature; dispatcher is captured via closure
- MUST update each of the nine command constructors (start, exec, fix-reviews, fetch-reviews, migrate, sync, archive, validate-tasks, setup) to build a typed kernel command from flags and call `kernel.Dispatch`
- MUST use the `*FromConfig(cfg core.Config)` translators from task_04 inside each closure
- MUST rewrite `core.Run`, `core.Prepare`, `core.FetchReviews`, `core.Migrate`, `core.Sync`, `core.Archive` in `internal/core/api.go` as thin adapters that build typed commands and call `kernel.Dispatch` (this preserves any external callers while redirecting all work through the kernel)
- MUST keep `core.Config` struct exports intact during Phase A (marked transitional in doc comment)
- MUST generate `docs/events.md` describing event taxonomy per kind with payload fields
- MUST write `docs/reader-library.md` with tested usage examples for List, Open, Replay, Tail, WatchWorkspace
- MUST verify `make verify` passes end-to-end after all changes
- MUST NOT regress any existing CLI stdout contract, JSON output shape, or artifact file production
</requirements>

## Subtasks
- [ ] 8.1 Add kernel bootstrap in `root.go`: construct bus, KernelDeps, dispatcher at command setup
- [ ] 8.2 Update `newCommandState` / command constructors to accept and capture the dispatcher
- [ ] 8.3 Refactor `runWorkflow` closure in each of the nine command files to build typed command + call Dispatch
- [ ] 8.4 Rewrite `internal/core/api.go` exported functions as thin dispatcher adapters
- [ ] 8.5 Mark `core.Config` as transitional in doc comments
- [ ] 8.6 Generate `docs/events.md` from `pkg/compozy/events/kinds/` struct definitions
- [ ] 8.7 Write `docs/reader-library.md` with copy-pasteable, tested examples
- [ ] 8.8 Run `make verify` and resolve any issues
- [ ] 8.9 Update integration tests for stable external contracts per ADR and TechSpec "CLI → kernel parity" test definition

## Implementation Details
See TechSpec "Build Order" steps 12-14 and ADR-001 "Implementation Notes" for the CLI injection pattern (dispatcher captured via closure, `commandState.runWorkflow` signature preserved). The nine command files at `internal/cli/` follow the same mechanical pattern: each closure calls `kernel.Dispatch[CommandType, ResultType](ctx, dispatcher, commands.FromConfig(cfg))`. `core.Config` stays as a translation shape during Phase A and is scheduled for removal in a later phase.

### Relevant Files
- `internal/cli/root.go:72-107` — `NewRootCommand` registers all nine commands; add KernelDeps construction here
- `internal/cli/root.go:886` — `commandState.runWorkflow` injection seam signature to preserve
- `internal/cli/*.go` — nine command constructor files: start, exec, fix-reviews, fetch-reviews, migrate, sync, archive, validate-tasks, setup
- `internal/core/api.go:202-251` — six exported functions becoming thin dispatcher adapters
- `internal/core/api.go:70` — `core.Config` struct with ~30 fields (marked transitional)
- `pkg/compozy/events/kinds/` (task_01) — source of truth for docs/events.md generation
- `pkg/compozy/runs/` (task_07) — API surface documented in docs/reader-library.md
- `internal/cli/root_command_execution_test.go:90` — existing stdout/JSON/artifact contract tests to preserve

### Dependent Files
- All existing CLI command integration tests — must continue passing against stable external contracts
- Any external Go callers of `core.Run`/`core.Prepare`/etc. (within the repo) — behavior unchanged by the adapter rewrite
- `CLAUDE.md` — update with new package paths (kernel, events, runs, journal)

### Related ADRs
- [ADR-001: Service Kernel Pattern with Typed Per-Command Handlers](adrs/adr-001.md) — defines CLI injection path and `runWorkflow` closure pattern

## Deliverables
- `KernelDeps` + `kernel.BuildDefault` wired into `root.go`
- Nine command constructors refactored to dispatch through kernel
- `internal/core/api.go` exports rewritten as thin dispatcher adapters
- `docs/events.md` covering all event kinds with payload schemas
- `docs/reader-library.md` with runnable usage examples
- `make verify` passes at 100%
- Integration tests asserting stable external contracts **(REQUIRED)**
- Test coverage >=80% across refactored CLI code **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `runWorkflow` closure for `start` command builds `RunStartCommand` with fields from flags and calls `kernel.Dispatch`
  - [ ] `runWorkflow` closure for `fix-reviews` passes `Mode=ModePRReview` in the command
  - [ ] `runWorkflow` closure for `exec` builds command with prompt text/file/stdin source correctly
  - [ ] `runWorkflow` closure for `migrate` builds `WorkspaceMigrateCommand` from migration flags
  - [ ] `runWorkflow` closure for `sync` builds `WorkflowSyncCommand` from sync flags
  - [ ] `runWorkflow` closure for `archive` builds `WorkflowArchiveCommand` from archive flags
  - [ ] `runWorkflow` closure for `fetch-reviews` builds `ReviewsFetchCommand` with provider+PR+round
  - [ ] `core.Run(ctx, cfg)` adapter dispatches `RunStartCommand` and returns dispatcher error unchanged
  - [ ] `core.Prepare(ctx, cfg)` adapter dispatches `WorkflowPrepareCommand` and returns Preparation
  - [ ] Registry self-test (from task_04) is invoked at CLI startup and fails build if any handler is missing
- Integration tests:
  - [ ] CLI → kernel parity per TechSpec definition: run `compozy start --name ...` end-to-end; assert exit code, stdout JSON shape (when --format json), presence and content of run.json, result.json, events.jsonl, per-job .prompt.md, .out.log, .err.log
  - [ ] CLI → kernel parity for `compozy exec` with --format json assertions match pre-refactor baseline
  - [ ] CLI → kernel parity for `compozy fix-reviews` assertions match pre-refactor baseline
  - [ ] `make verify` (fmt + lint + test + build) passes end-to-end
  - [ ] `docs/reader-library.md` examples compile and execute successfully (doc tests or standalone example files)
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Zero golangci-lint issues
- `make verify` passes at 100%
- `commandState.runWorkflow` signature preserved (`func(context.Context, core.Config) error`)
- No regression in `internal/cli/root_command_execution_test.go` or `execution_ui_test.go` external-contract assertions
- `docs/events.md` enumerates every event kind defined in `pkg/compozy/events/kinds/`
- `docs/reader-library.md` examples compile without modifications
