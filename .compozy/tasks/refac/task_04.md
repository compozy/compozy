---
status: completed
title: "Phase 3: Package-level splits"
type: refactor
complexity: critical
dependencies:
  - task_02
  - task_03
---

# Task 04: Phase 3: Package-level splits

## Overview

Decompose the God Package `internal/core/run/` (17 files, 8,700+ lines, 7+ responsibilities) into focused sub-packages, and extract the migration sub-package from `core`. This is the most impactful structural change in the entire refactoring -- it transforms a monolith where any change risks all 7 execution domains into an architecture where each concern evolves independently.

<critical>
- ALWAYS READ the TechSpec (20260406-summary.md) and detailed reports before starting
- REFERENCE 20260406-agent-run.md for the proposed package structure (F1) and per-file assignments
- FOCUS ON "WHAT" — create new packages, move code, update imports
- MINIMIZE CODE — pure structural moves; do not refactor logic during the move
- TESTS REQUIRED — run `make verify` after each sub-package extraction
</critical>

<requirements>
- MUST extract `internal/core/run/exec/` from `exec_flow.go` and related exec-mode types: `execRunState`, `PersistedExecRun`, `persistedExecTurn`, `execEventEmitter`, `execEventWriter`, `shouldRetryExecAttempt`, and the `ExecuteExec` entry point (G3-F3)
- MUST extract `internal/core/run/ui/` from all `ui_*` files: `ui_model.go`, `ui_update.go`, `ui_view.go` (already split into panel files), `ui_layout.go`, `ui_styles.go`, `ui_adapter_test.go`, and `validation_form.go` (G3-F1)
- MUST extract `internal/core/run/executor/` from `execution.go` (already split into focused files): job lifecycle, shutdown orchestration, job runner, session execution, retry logic, review hooks (G3-F1, G3-F2)
- MUST extract `internal/core/run/transcript/` from `session_view_model.go` and content-block rendering extracted from `logging.go` (G3-F1)
- MUST create `internal/core/contentconv/` for bidirectional `model` <-> `kinds` content-block and session-update conversion, eliminating the duplicated switch statements in `events.go` and `ui_model.go` (G3-F4)
- MUST extract `internal/core/migration/` from `core/migrate.go` (426 lines of V1-to-V2 format conversion), leaving only a thin `MigrateDirect` forwarding function in `core` (G2-F8)
- MUST keep `internal/core/run/` as a thin facade with `Execute()` and `ExecuteExec()` entry points that delegate to `executor/` and `exec/`
- MUST keep shared types (config, job, failInfo, phases) in `run/` accessible to sub-packages
- MUST NOT introduce circular dependencies between new sub-packages
- MUST pass `make verify` with zero issues
</requirements>

## Subtasks

- [x] 4.1 Create `internal/core/contentconv/` with bidirectional content-block and session-update converters (prerequisite for ui/ and executor/)
- [x] 4.2 Extract `internal/core/run/exec/` package from exec_flow files
- [x] 4.3 Extract `internal/core/run/transcript/` from session_view_model and render_blocks
- [x] 4.4 Extract `internal/core/run/ui/` from all ui_* files (depends on transcript/ for session view model)
- [x] 4.5 Extract `internal/core/run/executor/` from execution files (depends on exec/, ui/ being separate)
- [x] 4.6 Extract `internal/core/migration/` from core/migrate.go

## Implementation Details

Execute in the order listed. Each extraction follows the same pattern: create new package directory, move files with updated package declaration, update imports in all callers, verify compilation.

The target structure after this task:

```
run/
  run.go              — public Execute()/ExecuteExec() facade
  config.go           — shared config + job types
  types.go            — shared runtime types (failInfo, phases)
  result.go           — result building
  events.go           — thin event emission (uses contentconv)

run/executor/         — batch job execution engine
run/exec/             — headless exec-mode state machine
run/ui/               — Bubble Tea TUI components
run/transcript/       — session view model + content rendering
run/journal/          — (unchanged) durable event journal

contentconv/          — bidirectional model <-> kinds conversion
migration/            — V1-to-V2 workspace migration
```

### Relevant Files

- `internal/core/run/exec_flow.go` (and files split from it in Phase 1) — source for `run/exec/`
- `internal/core/run/execution.go` (and files split from it in Phase 1) — source for `run/executor/`
- `internal/core/run/ui_model.go`, `ui_update.go`, `ui_view.go`, `ui_layout.go`, `ui_styles.go`, `validation_form.go` — source for `run/ui/`
- `internal/core/run/session_view_model.go`, render_blocks from `logging.go` — source for `run/transcript/`
- `internal/core/run/events.go` — `publicContentBlock`/`publicSessionUpdate` to move to `contentconv/`
- `internal/core/run/ui_model.go` — `internalContentBlock`/`internalSessionUpdate` to move to `contentconv/`
- `internal/core/migrate.go` — 426 lines to move to `migration/`
- `internal/core/model/content.go` — content block types consumed by `contentconv/`
- `pkg/compozy/events/kinds/session.go` — event kinds consumed by `contentconv/`

### Dependent Files

- `internal/cli/root.go` — imports `run`, may need to import sub-packages for specific types
- `internal/core/api.go` — calls `run.Execute`, thin facade still works
- `internal/core/kernel/handlers.go` — calls execution functions, imports will change
- All test files in `run/` — must be moved to appropriate sub-packages
- `internal/core/migrate_test.go` — moves to `migration/`

## Deliverables

- 6 new packages created: `run/exec/`, `run/ui/`, `run/executor/`, `run/transcript/`, `contentconv/`, `migration/`
- `run/` reduced to a thin facade (<200 lines) plus shared types
- No duplicate content-block conversion code (`contentconv/` is the single source)
- Zero circular dependencies between packages
- `make verify` passes with zero issues **(REQUIRED)**

## Tests

- Unit tests:
  - [x] `contentconv.PublicContentBlock` produces identical output to the former `events.go:publicContentBlock`
  - [x] `contentconv.InternalContentBlock` produces identical output to the former `ui_model.go:internalContentBlock`
  - [x] `run.Execute()` facade delegates correctly to `executor.Execute()`
  - [x] `run.ExecuteExec()` facade delegates correctly to `exec.Execute()`
  - [x] All existing `run/` tests pass in their new sub-package locations
  - [x] All existing `migrate_test.go` tests pass in `migration/` package
  - [x] No import cycles are introduced by the new package graph
- Integration tests:
  - [x] `make verify` passes (fmt + lint + test + build)
  - [x] Exec flow integration tests pass from new `run/exec/` location
  - [x] ACP integration tests pass from new `run/executor/` location
- All tests must pass

## Success Criteria

- All tests passing
- `make verify` exits 0
- `internal/core/run/` has <200 lines of production code (excluding sub-packages)
- No import cycles are introduced by the package split
- `internal/core/migrate.go` reduced to a thin forwarding function (<30 lines)
- Zero duplicated content-block conversion code
