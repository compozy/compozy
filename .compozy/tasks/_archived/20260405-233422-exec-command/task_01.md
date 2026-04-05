---
status: completed
title: Runtime Mode & Workspace Config Surface
type: backend
complexity: high
dependencies: []
---

# Runtime Mode & Workspace Config Surface

## Overview

Introduce `exec` as a first-class execution mode in the shared runtime model and extend workspace configuration so the command can participate in the same precedence rules as existing workflows. No PRD exists for this feature, so the TechSpec and ADRs are the primary source of truth for the scope and contract of this task.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST add an `exec` execution mode to the shared runtime and API configuration types used by CLI, planning, and execution
- MUST add explicit output-format and ad hoc prompt-source fields to the shared runtime config so downstream layers do not infer behavior from CLI-only state
- MUST extend workspace config parsing and validation with an `[exec]` section that follows precedence `flags > [exec] > [defaults] > internal defaults`
- MUST keep existing `start`, `fix-reviews`, and `fetch-reviews` config behavior stable while adding `exec`
- MUST update runtime validation so the new mode is accepted and invalid mode-specific combinations fail with descriptive errors
</requirements>

## Subtasks
- [x] 1.1 Add shared model and API fields for `exec`, output format, and prompt-source metadata
- [x] 1.2 Extend workspace config structs, decoding, and validation to support `[exec]` defaults
- [x] 1.3 Apply `[exec]` defaults in the CLI config merge path without regressing existing command precedence
- [x] 1.4 Update runtime validation to recognize `exec` and reject invalid configuration combinations early
- [x] 1.5 Add unit and command-level tests for defaults, precedence, and validation behavior

## Implementation Details

Use the TechSpec sections "Core Interfaces", "Data Models", and "Component Overview" as the contract for the shared runtime shape. This task should establish the stable configuration surface that later tasks consume; it should not yet implement artifact layout or the `exec` command itself.

### Relevant Files
- `internal/core/model/model.go` — Shared execution mode constants, runtime config fields, and workspace path helpers live here
- `internal/core/api.go` — Public internal config facade and runtime conversion need the new mode and fields
- `internal/core/agent/registry.go` — Runtime validation currently restricts supported modes and needs to admit `exec`
- `internal/core/workspace/config.go` — Workspace config structs and strict validation live here
- `internal/cli/workspace_config.go` — CLI-level config merge precedence is applied here
- `internal/core/model/model_test.go` — Existing model default and path helper patterns should be extended
- `internal/core/workspace/config_test.go` — Config parsing and precedence regressions should be covered here

### Dependent Files
- `internal/cli/root.go` — The new `exec` command will consume the config surface defined by this task
- `internal/core/plan/input.go` — Prompt-backed planning will rely on the new mode and prompt-source fields
- `internal/core/run/types.go` — Executor config projection will depend on the new output-format fields

### Related ADRs
- [ADR-001: Model Ad Hoc Execution as a First-Class Runtime Mode](../adrs/adr-001.md) — Defines `exec` as a shared runtime mode instead of a parallel path
- [ADR-003: Support Multi-Source Prompt Input and Structured Output for `compozy exec`](../adrs/adr-003.md) — Defines prompt-source fields, output format, and config precedence

## Deliverables
- Shared runtime and API config support for `ExecutionModeExec`
- Workspace config support for `[exec]` and merged precedence behavior
- Runtime validation updates for the new mode and format fields
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for config precedence and validation behavior **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `RuntimeConfig.ApplyDefaults()` preserves existing defaults and initializes `exec`-specific fields safely
  - [x] Workspace config decoding accepts `[exec]` and rejects unknown or invalid keys with descriptive errors
  - [x] Config precedence resolves `flags > [exec] > [defaults] > internal defaults` for IDE, model, and output format
  - [x] Runtime validation accepts `exec` and rejects unsupported output formats or missing prompt-source intent
- Integration tests:
  - [x] CLI config application keeps `start` and `fix-reviews` behavior unchanged after adding `[exec]`
  - [x] A constructed `core.Config` for `exec` converts into a valid runtime config accepted by shared validation
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `exec` is represented as a first-class mode in shared config and validation layers
- `[exec]` config can be loaded without regressing existing workspace config behavior
- Downstream tasks can consume prompt-source and output-format fields without CLI-specific branching
