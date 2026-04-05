---
status: completed
title: Unified Run Artifact Layout & Existing Mode Migration
type: refactor
complexity: high
dependencies:
  - task_01
---

# Unified Run Artifact Layout & Existing Mode Migration

## Overview

Replace the legacy `.tmp/codex-prompts` artifact path with a shared workspace-scoped `.compozy/runs/<run-id>/` layout that works for all execution modes. This task is a runtime refactor with standalone value: even before `exec` ships, existing `start` and `fix-reviews` runs should already allocate artifacts through the new unified layout.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST retire `.tmp/codex-prompts` as the canonical runtime artifact root for newly prepared runs
- MUST introduce shared workspace-scoped helpers for `.compozy/runs/<run-id>/` and per-run artifact allocation
- MUST move current directory-backed run preparation onto the new artifact layout without changing their execution semantics
- MUST produce deterministic seams for tests so run directories and per-job artifact paths can be asserted reliably
- MUST avoid auto-migrating historical `.tmp/codex-prompts` directories in this task
</requirements>

## Subtasks
- [x] 2.1 Introduce shared run-artifact path helpers and run-directory allocation under `.compozy/runs/`
- [x] 2.2 Refactor planning to allocate run-level metadata and job artifact paths through the new helpers
- [x] 2.3 Update existing PRD-task and review-mode preparation to use the new artifact root
- [x] 2.4 Remove remaining hardcoded assumptions that the artifact root is `.tmp/codex-prompts`
- [x] 2.5 Add regression tests that prove existing modes now write artifacts under `.compozy/runs/`

## Implementation Details

Use the TechSpec sections "Run artifact layout", "Component Overview", and "Development Sequencing" as the design guide. This task should create the shared platform that both existing modes and the later `exec` feature consume. Keep this task focused on artifact allocation and path policy, not prompt-source resolution or JSON output.

### Relevant Files
- `internal/core/model/model.go` — Shared workspace path helpers should expose the new runs base directory
- `internal/core/plan/input.go` — The current artifact root is hardcoded here and must be replaced
- `internal/core/plan/prepare.go` — Job artifact allocation is currently tied to prompt writing here
- `internal/core/plan/prepare_test.go` — Existing planning tests should be updated to assert the new artifact root
- `internal/core/api.go` — Preparation results expose artifact paths to callers and tests
- `internal/core/run/types.go` — Executor-facing job config mirrors prompt/log paths and should remain consistent
- `internal/core/run/ui_update.go` — UI job metadata should continue to show valid log paths after the layout change

### Dependent Files
- `internal/core/run/command_io.go` — Session log writers consume the per-job out/err paths created by this task
- `internal/core/run/execution_acp_test.go` — Execution tests should be updated to use the new artifact conventions where applicable
- `internal/core/run/ui_model.go` — UI queueing still depends on accurate prompt/log metadata coming from planning

### Related ADRs
- [ADR-002: Replace `.tmp/codex-prompts` with Workspace-Scoped Run Artifacts](../adrs/adr-002.md) — Defines the new artifact root and shared run layout

## Deliverables
- Shared `.compozy/runs/` path helpers and run-directory allocation logic
- Planning updates that write prompts and logs under the new run layout for existing modes
- Removal of new-write reliance on `.tmp/codex-prompts`
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests proving existing modes allocate artifacts under `.compozy/runs/` **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Workspace path helpers return `.compozy/runs/` paths relative to the discovered workspace root
  - [x] Run-directory allocation produces stable job prompt/log paths within one run directory
  - [x] Planning writes prompt artifacts under `.compozy/runs/<run-id>/jobs/` for PRD task mode
  - [x] Planning writes prompt artifacts under `.compozy/runs/<run-id>/jobs/` for review mode
- Integration tests:
  - [x] Preparing a `start` workflow creates prompt and log paths under `.compozy/runs/` without changing task ordering behavior
  - [x] Preparing a review workflow creates prompt and log paths under `.compozy/runs/` without changing review filtering behavior
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Newly prepared runs no longer allocate artifacts under `.tmp/codex-prompts`
- Existing execution modes can prepare and expose artifacts from `.compozy/runs/` without behavior regressions
- The shared artifact model is ready for `exec` to build on directly
