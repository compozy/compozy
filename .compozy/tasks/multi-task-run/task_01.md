---
status: completed
title: Add Parallel Limit Workspace Configuration
type: backend
complexity: medium
dependencies: []
---

# Task 1: Add Parallel Limit Workspace Configuration

## Overview

This task adds the workspace configuration foundation for bounded parallel multi-run execution. It introduces the configurable fanout limit while preserving `enqueued` as the default multi-run mode.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The workspace config MUST accept `[tasks.run] run_multiple_parallel_limit`.
- The effective parallel limit MUST default to `2` when unset.
- Config validation MUST reject zero and negative `run_multiple_parallel_limit` values.
- Global and workspace config merge behavior MUST follow existing `[tasks.run]` precedence.
- Existing `run_multiple_mode` defaults and validation MUST remain compatible.
- This task MUST NOT change CLI or daemon scheduling behavior.
</requirements>

## Subtasks

- [x] 1.1 Add the parallel limit field to the task-run config model.
- [x] 1.2 Add an effective limit helper with default `2`.
- [x] 1.3 Merge global and workspace parallel limit values.
- [x] 1.4 Validate positive configured limit values.
- [x] 1.5 Add config parsing, merge, default, and validation tests.

## Implementation Details

Use the TechSpec "CLI And Configuration" section and ADR-008 for default and precedence rules. Keep this task limited to configuration behavior so later CLI and daemon work can consume a stable helper.

### Relevant Files

- `internal/core/workspace/config_types.go` — defines `TaskRunConfig` and effective config helpers.
- `internal/core/workspace/config_merge.go` — merges global and workspace `[tasks.run]` values.
- `internal/core/workspace/config_validate.go` — validates task-run config fields.
- `internal/core/workspace/config_test.go` — covers config parsing, merge, defaults, and validation.

### Dependent Files

- `internal/cli/daemon_commands.go` — later task consumes the effective limit for CLI request wiring.
- `internal/api/contract/types.go` — later task adds the daemon request field receiving the resolved limit.
- `README.md` — later documentation describes the config option and default.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Requires bounded parallel execution.
- [ADR-008: Make the Parallel Multi-Run Limit Configurable](adrs/adr-008.md) — Defines the config key, default, and precedence.

## Deliverables

- `run_multiple_parallel_limit` parsed, merged, defaulted, and validated.
- Focused config tests for positive, zero, negative, unset, and merged values.
- Unit tests with 80%+ coverage for changed config helpers **(REQUIRED)**.
- Integration-oriented config load tests for TOML examples **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Loading `[tasks.run] run_multiple_parallel_limit = 2` stores the configured value.
  - [x] Unset `run_multiple_parallel_limit` returns effective limit `2`.
  - [x] `run_multiple_parallel_limit = 0` returns a validation error naming the config field.
  - [x] `run_multiple_parallel_limit = -1` returns a validation error naming the config field.
  - [x] Workspace config overrides global config for the parallel limit.
- Integration tests:
  - [x] Loading global plus workspace TOML preserves existing `run_multiple_mode` behavior and applies the new limit precedence.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Config exposes a positive parallel fanout limit with default `2`.
- Existing multi-run mode config behavior is unchanged.
