---
status: pending
title: Add Multi-Run Config and Slug Parsing Foundations
type: backend
complexity: medium
dependencies: []
---

# Task 1: Add Multi-Run Config and Slug Parsing Foundations

## Overview

This task adds the configuration and input parsing foundations required by `tasks run-multiple`. It keeps the existing `tasks run` path stable while introducing shared validation helpers that later CLI, API, and daemon work can rely on.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The implementation MUST add `[tasks.run] run_multiple_mode` to the workspace config model.
- The implementation MUST merge global and workspace config using the same precedence pattern as the existing `[tasks.run]` fields.
- The implementation MUST validate only `enqueued` and `parallel` as accepted V1 values.
- The implementation MUST expose a default effective mode of `enqueued` when no config value is present.
- The implementation MUST provide a reusable comma-separated slug parser that trims entries, preserves order, rejects empty entries, and rejects duplicates.
- The implementation MUST NOT change the existing `compozy tasks run <slug>` command behavior.
</requirements>

## Subtasks

- [ ] 1.1 Add the `run_multiple_mode` field to the task-run config model.
- [ ] 1.2 Add merge and validation behavior for the new config field.
- [ ] 1.3 Add a small parser for comma-separated multi-run slug input.
- [ ] 1.4 Add unit tests for config loading, merge precedence, invalid modes, parser errors, and duplicate slugs.
- [ ] 1.5 Confirm existing single-run config tests still pass unchanged.

## Implementation Details

Update the config layer first, then place the slug parser near the future CLI command wiring so later tasks can reuse it without importing daemon internals. See the TechSpec "Data Models" and "Development Sequencing" sections for the expected field name and accepted values.

### Relevant Files

- `internal/core/workspace/config_types.go` — defines `TaskRunConfig`, which must accept `run_multiple_mode`.
- `internal/core/workspace/config_merge.go` — merges global and workspace `[tasks.run]` values.
- `internal/core/workspace/config_validate.go` — validates task-run config fields and should reject unsupported modes.
- `internal/core/workspace/config_test.go` — covers config parsing, validation, and merge behavior.
- `internal/cli/daemon_commands.go` — current task command file and likely home for the parser or parser call site.
- `internal/cli/commands_test.go` — existing command-level tests for task-run defaults.

### Dependent Files

- `internal/cli/workspace_config.go` — later tasks will apply the new config value to the multi-run command state.
- `internal/cli/workspace_config_test.go` — later tests should assert config defaults are applied to `run-multiple`.
- `README.md` — later documentation should mention the config key.

### Related ADRs

- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md) — Keeps this work off the existing single-run command.
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md) — Defines the config key, default mode, and accepted `parallel` fallback.

## Deliverables

- `run_multiple_mode` parsed, merged, and validated in workspace config.
- Reusable comma-separated slug parser with order-preserving validation.
- Unit tests with 80%+ coverage for the changed config and parser behavior **(REQUIRED)**.
- Integration-oriented config tests that load workspace/global TOML combinations **(REQUIRED)**.

## Tests

- Unit tests:
  - [ ] Loading `[tasks.run] run_multiple_mode = "enqueued"` stores the configured value.
  - [ ] Loading `[tasks.run] run_multiple_mode = "parallel"` stores the configured value without error.
  - [ ] Loading `[tasks.run] run_multiple_mode = "invalid"` returns a validation error naming `tasks.run.run_multiple_mode`.
  - [ ] Workspace config overrides global config for `run_multiple_mode`.
  - [ ] Parsing `alpha,beta,gamma` returns slugs in that order.
  - [ ] Parsing `alpha,,beta` rejects the empty slug.
  - [ ] Parsing `alpha, beta ,alpha` rejects the duplicate `alpha`.
- Integration tests:
  - [ ] `workspace.LoadConfig` accepts the new key while still rejecting unknown task-run keys.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing.
- Test coverage >=80%.
- Config accepts `enqueued` and `parallel` only.
- Parser behavior is deterministic and reusable by later tasks.
- Existing `tasks run` tests remain green without behavioral changes.
