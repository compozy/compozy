---
status: completed
title: Update Documentation and End-to-End Coverage
type: docs
complexity: medium
dependencies:
  - task_08
  - task_09
---

# Task 10: Update Documentation and End-to-End Coverage

## Overview

This task finishes the regenerated parallel multi-run implementation plan with user-facing documentation and full-flow verification. It removes obsolete fallback wording, documents the configurable limit and preserved worktree handoff, and adds end-to-end coverage for true parallel behavior.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- Documentation MUST describe `compozy tasks run --multiple ... --parallel`.
- Documentation MUST describe `[tasks.run] run_multiple_parallel_limit = 2` and `--parallel-limit <n>`.
- Documentation MUST remove obsolete wording that says `parallel` falls back to enqueued mode.
- Documentation MUST explain what git worktrees isolate and what they do not isolate.
- Documentation MUST state that V1 preserves child worktrees and does not auto-merge, push, or delete them.
- End-to-end coverage MUST exercise true parallel mode and final worktree handoff.
- The final implementation MUST pass the full repository verification gate.
</requirements>

## Subtasks

- [x] 10.1 Update README command and config examples for true parallel mode.
- [x] 10.2 Document limit precedence and default `2`.
- [x] 10.3 Document preserved worktree handoff and V1 non-goals.
- [x] 10.4 Update event documentation for additive multi-run metadata fields.
- [x] 10.5 Add end-to-end coverage for parallel execution and final output.
- [x] 10.6 Run full project verification and capture evidence.

## Implementation Details

Use the TechSpec "Non-Goals", "UI And Output", and "Testing Approach" sections. Keep docs close to the existing `tasks run --multiple` and `[tasks.run]` sections so users see safe default, opt-in parallel mode, and review handoff together.

### Relevant Files

- `README.md` — primary CLI and config documentation.
- `docs/events.md` — public event payload documentation when present.
- `internal/cli/root_command_execution_test.go` — end-to-end CLI execution tests.
- `internal/cli/daemon_exec_test_helpers_test.go` — in-process daemon helper patterns.
- `web/e2e/daemon-ui.smoke.spec.ts` — daemon UI smoke coverage if visible behavior changes.
- `Makefile` — full verification entrypoint.

### Dependent Files

- `internal/cli/daemon_commands.go` — final CLI behavior documented by this task.
- `internal/daemon/task_multi.go` — final scheduler behavior verified by this task.
- `internal/core/run/ui/multi_remote.go` — final TUI handoff behavior documented by this task.
- `openapi/compozy-daemon.json` — schema artifact updated by earlier contract work.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Defines true parallel and worktree isolation boundaries.
- [ADR-006: Use a Speed-First Parallel MVP for the PRD Scope](adrs/adr-006.md) — Defines the MVP promise and handoff.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Explains canonical child visibility.
- [ADR-008: Make the Parallel Multi-Run Limit Configurable](adrs/adr-008.md) — Defines configurable limit docs and precedence.

## Deliverables

- README updates for true parallel multi-run usage.
- Documentation for config and CLI limit precedence.
- Documentation for worktree preservation and isolation limits.
- Event documentation for additive metadata fields.
- End-to-end tests for true parallel execution and final handoff output.
- Unit tests with 80%+ coverage for any changed helper or formatter code **(REQUIRED)**.
- Integration or e2e tests for the documented parallel multi-run flow **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Any new documentation formatter or output helper has focused coverage.
  - [x] Event documentation examples match current `TaskRunMultiplePayload` field names.
- Integration tests:
  - [x] End-to-end `tasks run --multiple alpha,beta --parallel` starts a parent and reports child worktree paths.
  - [x] End-to-end `--parallel-limit 1` respects bounded execution and final handoff output.
  - [x] README command snippets remain aligned with actual CLI help.
  - [x] Full `make verify` passes after all implementation and docs changes.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- User-facing docs describe true parallel behavior and no longer mention fallback execution.
- End-to-end coverage protects the full parallel multi-run workflow.
