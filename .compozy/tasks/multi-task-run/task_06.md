---
status: completed
title: Document Multi-Run Usage and Add End-to-End Coverage
type: docs
complexity: medium
dependencies:
  - task_04
  - task_05
---

# Task 6: Document Multi-Run Usage and Add End-to-End Coverage

## Overview

This task finishes the feature by documenting the user-facing command, config behavior, fallback semantics, and quit dialog behavior. It also adds end-to-end coverage for the complete multi-run flow so the command, daemon coordinator, and TUI-facing state remain protected.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- Documentation MUST explain when to use `tasks run` versus `tasks run-multiple`.
- Documentation MUST show the comma-separated input shape.
- Documentation MUST document `[tasks.run] run_multiple_mode = "enqueued"` as the default behavior.
- Documentation MUST explain that `parallel` is accepted in V1 but falls back to enqueued with V2 worktree messaging.
- Documentation MUST describe tab states and the existing `Close TUI`, `Stop Run`, and `Cancel` quit behavior as applied to the queue.
- End-to-end coverage MUST exercise multiple slugs in one invocation.
- The final implementation MUST pass `make verify`.
</requirements>

## Subtasks

- [x] 6.1 Add README or command documentation for `tasks run-multiple`.
- [x] 6.2 Add config examples for `run_multiple_mode`.
- [x] 6.3 Document V1 `parallel` fallback and V2 worktree expectation.
- [x] 6.4 Document queue-level quit dialog behavior.
- [x] 6.5 Add end-to-end or smoke coverage for multi-run command execution with multiple slugs.
- [x] 6.6 Run and document full project verification.

## Implementation Details

Keep documentation near existing `tasks run` and `[tasks.run]` config guidance so users can compare single-run and multi-run behavior. The end-to-end test should use disposable task workflows and must verify the exact comma-separated input shape.

### Relevant Files

- `README.md` — primary user-facing CLI and config documentation.
- `internal/cli/root_command_execution_test.go` — end-to-end CLI execution patterns against temp workspaces.
- `internal/cli/daemon_exec_test_helpers_test.go` — in-process daemon helper patterns for CLI tests.
- `web/e2e/daemon-ui.smoke.spec.ts` — existing daemon UI smoke coverage if web-visible run state changes are surfaced.
- `Makefile` — final verification entrypoint.

### Dependent Files

- `internal/cli/daemon_commands.go` — final CLI command behavior documented by this task.
- `internal/daemon/run_manager.go` — final daemon parent/child behavior verified by this task.
- `internal/core/run/ui` — final tab behavior documented by this task.
- `openapi/compozy-daemon.json` — may change if API contracts are regenerated as part of prior tasks.

### Related ADRs

- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md) — Documents the separate command decision.
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md) — Documents config, fallback, and tabs.
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) — Documents queue-level detach/stop behavior.

## Deliverables

- User-facing documentation for `compozy tasks run-multiple`.
- Config documentation for `run_multiple_mode` and V1 `parallel` fallback.
- End-to-end test coverage for multiple slugs in one invocation.
- Unit tests with 80%+ coverage for any changed helper code **(REQUIRED)**.
- Integration or e2e tests for the documented multi-run flow **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Any new documentation helper or formatter code has focused unit coverage.
  - [x] Any changed command help text tests assert `run-multiple` examples are present.
- Integration tests:
  - [x] CLI e2e starts two task workflows using `tasks run-multiple alpha,beta`.
  - [x] CLI e2e verifies both requested slugs appear in parent queue state.
  - [x] CLI e2e verifies `parallel` config fallback messaging appears and execution remains enqueued.
  - [x] Full `make verify` passes after docs and tests.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing.
- Test coverage >=80%.
- Documentation clearly distinguishes `tasks run` and `tasks run-multiple`.
- End-to-end coverage exercises comma-separated multi-slug input.
- `make verify` passes with zero lint issues.
