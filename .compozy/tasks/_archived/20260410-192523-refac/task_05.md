---
status: completed
title: "Phase 4: DRY and generics consolidation"
type: refactor
complexity: high
dependencies:
  - task_03
  - task_04
---

# Task 05: Phase 4: DRY and generics consolidation

## Overview

Leverage Go generics and shared abstractions to eliminate the remaining systemic duplication patterns across the codebase. This includes unifying the massive content-block type hierarchy duplication between `model/content.go` and `kinds/session.go`, replacing 20+ copy-paste function variants with generic helpers, collapsing the triple config translation chain, and extracting shared base structs in the CLI layer.

<critical>
- ALWAYS READ the TechSpec (20260406-summary.md) and detailed reports before starting
- REFERENCE 20260406-provider-public.md F01 for content block unification strategy
- REFERENCE 20260406-core-foundation.md F2 for config chain collapse strategy
- REFERENCE 20260406-cli-entry.md F2, F3, F4 for CLI generics
- FOCUS ON "WHAT" — introduce generics and shared abstractions to eliminate duplication
- MINIMIZE CODE — the goal is less code, not more abstraction
- TESTS REQUIRED — run `make verify` after each consolidation
</critical>

<requirements>
- MUST unify `internal/core/model/content.go` and `pkg/compozy/events/kinds/session.go` via a shared generic content-block engine — extract shared decode/encode/validate/ensure logic parameterized by JSON tag strategy (G5-F01)
- MUST introduce generic `applyConfig[T]` in `internal/cli/workspace_config.go` replacing 5 type-specific `applyStringConfig`/`applyIntConfig`/etc. functions (G1-F4)
- MUST introduce generic `applyInput[T]` in `internal/cli/form.go` replacing 4 type-specific `applyStringInput`/etc. functions (G1-F4)
- MUST introduce generic `decodeBlock[T]` in `internal/core/model/content.go` replacing 6 structurally identical `decodeXxxBlock` functions (G2-F4)
- MUST introduce generic `ensureBlockType[T]` or inline the ensure logic into the generic decode (G2-F4)
- MUST introduce generic dispatch adapter in `internal/core/kernel/core_adapters.go` replacing 6 near-identical adapter functions (G2-F5)
- MUST introduce generic delegating handler in `internal/core/kernel/handlers.go` for the 4 thin handlers (sync, archive, migrate, fetch) (G2-F7)
- MUST collapse the triple config translation chain: either embed `commands.RuntimeFields` in `core.Config` or accept `model.RuntimeConfig` directly in commands (G2-F2)
- MUST extract `simpleCommandBase` struct in `internal/cli/` for migrate/sync/archive command states, embedding shared fields and `loadWorkspaceRoot` (G1-F2)
- MUST replace agent spec closures in `internal/setup/agents.go` with a declarative table pattern (G5-F03)
- MUST introduce `SessionSetupRequest` parameter object in `internal/core/run/` (or `run/executor/`) replacing the 13-parameter `setupSessionExecution` (G3-F10)
- MUST introduce parameter object for `newSessionUpdateHandler` replacing its 13 parameters (G3-F11)
- MUST extract `commandState` sub-structs: `workflowIdentity`, `runtimeConfig`, `execConfig`, `retryConfig`, `commandStateCallbacks` (G1-F3)
- MUST extract generic `selectByName[T]` in `internal/setup/` replacing duplicate `SelectSkills`/`SelectAgents` (G5-F12)
- MUST NOT change external behavior — all changes are internal structural improvements
- MUST pass `make verify` with zero issues
</requirements>

## Subtasks

- [x] 5.1 Unify content-block type hierarchies between `model/content.go` and `kinds/session.go` via shared generic engine
- [x] 5.2 Introduce generic CLI helpers: `applyConfig[T]`, `applyInput[T]`, `simpleCommandBase`, `commandState` sub-structs
- [x] 5.3 Introduce generic kernel helpers: `decodeBlock[T]`, generic dispatch adapter, generic delegating handler
- [x] 5.4 Collapse the triple config translation chain (`core.Config` -> `RuntimeFields` -> `RuntimeConfig`)
- [x] 5.5 Replace agent spec closures with declarative table and extract `selectByName[T]`
- [x] 5.6 Introduce parameter objects for `setupSessionExecution` and `newSessionUpdateHandler`

## Implementation Details

This phase benefits from Go 1.23+ generics (confirmed in `go.mod`). Each subtask replaces N copy-paste variants with 1 generic function. The content-block unification (5.1) is the largest item -- approach it by creating a shared internal package with the generic decode/encode machinery, then having both `model` and `kinds` delegate to it while maintaining their distinct JSON tag strategies.

For the config chain collapse (5.4), the recommended approach is to embed `commands.RuntimeFields` directly into `core.Config`, eliminating one conversion layer entirely. This depends on Phase 2 having moved result types to `model` first.

### Relevant Files

- `internal/core/model/content.go` — 6 decode + 6 ensure functions to replace with generics
- `pkg/compozy/events/kinds/session.go` — parallel type hierarchy to unify with model
- `internal/cli/workspace_config.go` — 5 `applyConfig` type variants (lines 123-157)
- `internal/cli/form.go` — 4 `applyInput` type variants (lines 464-498)
- `internal/cli/root.go` — `commandState` (44 fields), `simpleCommandBase` extraction, `run()`/`exec()` consolidation
- `internal/core/kernel/core_adapters.go` — 6 near-identical adapter functions (lines 47-148)
- `internal/core/kernel/handlers.go` — 4 thin handler structs to replace with generic
- `internal/core/kernel/commands/runtime_fields.go` — 32-field `RuntimeFields` to embed or collapse
- `internal/core/api.go` — `Config.runtime()` conversion to simplify (lines 354-391)
- `internal/setup/agents.go` — 435-line agent spec slab to make declarative
- `internal/setup/install.go` — `SelectSkills` to generify
- `internal/setup/agents.go` — `SelectAgents` to generify
- `internal/core/run/command_io.go` — `setupSessionExecution` 13-param function (line 94)
- `internal/core/run/logging.go` (or `session_handler.go`) — `newSessionUpdateHandler` 13-param constructor

### Dependent Files

- All test files for modified packages
- `internal/cli/*_test.go` — affected by CLI generic helpers and sub-structs
- `internal/core/kernel/*_test.go` — affected by generic handlers and adapters
- `internal/core/run/*_test.go` — affected by parameter objects

## Deliverables

- Content-block types unified via shared engine (~450 lines of duplication eliminated)
- 20+ copy-paste function variants replaced by generic equivalents
- Config translation chain reduced from 3 hops to 1-2
- `commandState` decomposed into 5 focused sub-structs
- Agent specs converted to declarative table
- Parameter objects replace 2 functions with 13+ parameters each
- `make verify` passes with zero issues **(REQUIRED)**

## Tests

- Unit tests:
  - [x] Generic `applyConfig[T]` handles string, int, float64, bool, and []string types correctly
  - [x] Generic `decodeBlock[T]` correctly decodes all 6 block types with type validation
  - [x] Content-block unification preserves JSON serialization compatibility for both camelCase and snake_case
  - [x] Generic dispatch adapter produces identical results to the 6 individual adapters
  - [x] Generic delegating handler produces identical results to the 4 thin handlers
  - [x] `simpleCommandBase.loadWorkspaceRoot` works for migrate, sync, and archive commands
  - [x] `commandState` sub-structs correctly map to `core.Config` via `buildConfig()`
  - [x] Declarative agent spec table produces identical `agentSpecs` as the closure-based version
  - [x] `selectByName[T]` handles skills and agents with deduplication and alias resolution
  - [x] `SessionSetupRequest` parameter object correctly initializes session execution
  - [x] Collapsed config chain produces identical `RuntimeConfig` output
- Integration tests:
  - [x] `make verify` passes (fmt + lint + test + build)
  - [x] JSON round-trip tests for both camelCase and snake_case content blocks
- All tests must pass

## Success Criteria

- All tests passing
- `make verify` exits 0
- Zero structurally duplicate `applyConfig`/`applyInput` functions in CLI
- Zero structurally duplicate `decode*Block`/`ensure*Block` functions in model
- Zero structurally duplicate dispatch adapter functions in kernel
- Content-block decode/encode logic exists in exactly one location
- The targeted high-arity helpers in this phase no longer expose 13-parameter signatures
- Adding a new config field requires changes in at most 2 files (down from 4)
- Adding a new content block type requires changes in at most 2 files (down from 4+)
