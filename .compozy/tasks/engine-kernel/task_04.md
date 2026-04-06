---
status: pending
title: Service Kernel with typed commands
type: refactor
complexity: medium
dependencies:
  - task_01
---

# Task 04: Service Kernel with typed commands

## Overview
Create `internal/core/kernel/` with a generic typed-command dispatcher, `KernelDeps` aggregate, and `BuildDefault` factory that instantiates and registers the six Phase A command handlers (RunStart, WorkflowPrepare, WorkflowSync, WorkflowArchive, WorkspaceMigrate, ReviewsFetch). This layer becomes the single execution seam that both the Cobra CLI and a future RPC ingress call.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<note>
**Greenfield approach**: This project is in alpha (`v0.x`). Prioritize clean architecture and code quality over backwards compatibility. Do not add compatibility shims, legacy adapters, or deprecation wrappers — replace existing code directly. Breaking changes are expected and acceptable.
</note>

<requirements>
- MUST create package `internal/core/kernel/` with `Dispatcher`, `Handler[C,R]` interface, `Register`, `Dispatch`, `NewDispatcher`
- MUST define `KernelDeps` struct holding `Logger *slog.Logger`, `EventBus *events.Bus[events.Event]`, `Workspace workspace.Context`, `AgentRegistry agent.Registry`
- MUST expose `BuildDefault(deps KernelDeps) *Dispatcher` that instantiates and registers all six Phase A handlers
- MUST use Go 1.23 generics for type-safe dispatch via `Dispatch[C, R](ctx, d, cmd)`
- MUST create `internal/core/kernel/commands/` subpackage with typed Command+Result structs for the six Phase A operations
- MUST provide `*FromConfig(cfg core.Config)` translator functions that map legacy `core.Config` fields to each command struct
- MUST have handler implementations that call existing `plan.Prepare`, `run.Execute`, `fetchReviews`, etc. unchanged (no rewrite of core execution logic)
- MUST fail with a typed error if a command is dispatched without a registered handler
- MUST be race-free: concurrent `Register` and `Dispatch` calls protected by `sync.RWMutex`
- MUST include a registry self-test asserting all six Phase A command types are registered at `BuildDefault` time
</requirements>

## Subtasks
- [ ] 4.1 Create `internal/core/kernel/dispatcher.go` with `Dispatcher`, `Handler[C,R]`, `Register`, `Dispatch`, `NewDispatcher`
- [ ] 4.2 Create `internal/core/kernel/deps.go` with `KernelDeps` struct and `BuildDefault` factory
- [ ] 4.3 Create `internal/core/kernel/commands/` with six Command+Result struct pairs (RunStart, WorkflowPrepare, WorkflowSync, WorkflowArchive, WorkspaceMigrate, ReviewsFetch)
- [ ] 4.4 Implement `*FromConfig(cfg core.Config)` translator for each command
- [ ] 4.5 Implement handler types that wrap calls to existing engine functions (plan.Prepare, run.Execute, fetchReviews, etc.)
- [ ] 4.6 Implement `BuildDefault` that registers all six handlers and verifies completeness via self-test
- [ ] 4.7 Write unit tests for dispatcher routing, registry self-test, concurrent safety, FromConfig translators

## Implementation Details
See TechSpec "Core Interfaces" section for the `Handler[C,R]` + `Dispatcher` skeleton and ADR-001 for the full decision on typed per-command handlers, command list split between Phase A and deferred, `KernelDeps` composition, and CLI injection path. Handlers delegate to existing engine code — they are thin adapters, not rewrites.

### Relevant Files
- `internal/core/api.go:202-251` — six exported functions becoming six Phase A commands (Prepare, Run, FetchReviews, Migrate, Sync, Archive)
- `internal/core/api.go:70` — existing `core.Config` struct fields that drive `FromConfig` translators
- `internal/core/plan/prepare.go:24` — `plan.Prepare` called by RunStart/WorkflowPrepare handlers
- `internal/core/run/execution.go:27` — `run.Execute` called by RunStart handler
- `internal/core/fetch.go:19` — `fetchReviews` called by ReviewsFetch handler
- `internal/core/migrate.go`, `sync.go`, `archive.go` — called by WorkspaceMigrate, WorkflowSync, WorkflowArchive handlers
- `internal/core/workspace/config.go` — `workspace.Context` carried in KernelDeps
- `internal/core/agent/registry.go` — `agent.Registry` carried in KernelDeps
- `pkg/compozy/events/bus.go` (task_01) — `events.Bus[events.Event]` carried in KernelDeps

### Dependent Files
- `internal/cli/root.go` (task_08) — will construct `KernelDeps` at startup and call `BuildDefault`
- `internal/cli/*.go` (task_08) — will capture `*Dispatcher` in `runWorkflow` closures
- `internal/core/run/execution.go` (task_05) — will receive event bus reference via KernelDeps plumbing

### Related ADRs
- [ADR-001: Service Kernel Pattern with Typed Per-Command Handlers](adrs/adr-001.md) — defines dispatcher pattern, KernelDeps, Phase A command inventory, CLI seam strategy

## Deliverables
- `internal/core/kernel/dispatcher.go` with `Dispatcher`, `Handler[C,R]`, `Register`, `Dispatch`
- `internal/core/kernel/deps.go` with `KernelDeps` and `BuildDefault`
- `internal/core/kernel/commands/` package with six Command+Result struct files and FromConfig translators
- Six handler implementations wrapping existing engine calls
- Registry self-test enforcing exhaustive Phase A handler registration
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration test invoking each of the six handlers through `Dispatch` with mocked deps **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Register+Dispatch routes `RunStartCommand` to `RunStart` handler and returns `RunStartResult`
  - [ ] Dispatch of unregistered command type returns typed error naming the command type
  - [ ] Dispatch with type mismatch between registered handler and call site returns typed error
  - [ ] Registry self-test fails if any of the six Phase A commands is NOT registered by BuildDefault
  - [ ] Registry self-test passes when all six are registered
  - [ ] Concurrent Register+Dispatch from 100 goroutines produces no race (with -race)
  - [ ] `FromConfig` translator correctly maps `core.Config.Mode`, `IDE`, `Model`, `Name` into `RunStartCommand`
  - [ ] `FromConfig` for WorkflowSync passes through `TasksDir`, `DryRun` fields
  - [ ] Each of the six `FromConfig` translators covers every field the legacy handler reads
- Integration tests:
  - [ ] Full dispatch path: construct `KernelDeps` with fakes, `BuildDefault`, dispatch `RunStartCommand`, assert handler invokes `plan.Prepare` and `run.Execute` with expected arguments
  - [ ] Dispatching all six commands sequentially with appropriate inputs succeeds against mocked deps
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Zero golangci-lint issues
- `go test -race` passes for kernel package
- Registry self-test enforces all six handlers registered
- Phase A command set matches exactly the current `core/api.go` exports (no new user-facing operations introduced)
