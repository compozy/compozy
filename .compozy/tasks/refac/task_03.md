---
status: pending
title: "Phase 2: Domain restructuring"
type: refactor
complexity: critical
dependencies:
  - task_02
---

# Task 03: Phase 2: Domain restructuring

## Overview

Fix the most critical architectural inversions in the codebase by relocating domain parsing logic to its correct packages and breaking upward dependency chains. The `prompt` package currently acts as a misplaced domain parsing layer imported by 8+ packages for task/review parsing -- this task moves that parsing to `tasks` and `reviews` where it belongs. It also moves result/config types from `core` to `model` to break the `kernel` -> `core` and `commands` -> `core` upward dependencies.

<critical>
- ALWAYS READ the TechSpec (20260406-summary.md) and detailed reports before starting
- REFERENCE 20260406-plan-domain.md for parsing relocation details (F01, F09, F15)
- REFERENCE 20260406-core-foundation.md for type relocation details (F6, F13)
- FOCUS ON "WHAT" — relocate code to correct packages, update all callers
- MINIMIZE CODE — move existing code, do not rewrite logic
- TESTS REQUIRED — run `make verify` after each relocation to catch import breakage
</critical>

<requirements>
- MUST move task parsing functions from `prompt` to `tasks` package: `ParseTaskFile`, `ParseLegacyTaskFile`, `IsTaskCompleted`, `ExtractTaskNumber`, `LooksLikeLegacyTaskFile`, `ExtractLegacyTaskBody` (G4-F01)
- MUST move task-related error sentinels from `prompt` to `tasks`: `ErrLegacyTaskMetadata`, `ErrV1TaskMetadata` (G4-F15)
- MUST move review parsing functions from `prompt` to `reviews` package: `ParseReviewContext`, `ParseLegacyReviewContext`, `IsReviewResolved`, `ExtractIssueNumber`, `LooksLikeLegacyReviewFile`, `ExtractLegacyReviewBody` (G4-F01)
- MUST move review-related error sentinel from `prompt` to `reviews`: `ErrLegacyReviewMetadata` (G4-F15)
- MUST update all task-parsing callers across the codebase: `plan/input.go`, `plan/prepare.go`, `tasks/store.go`, `tasks/validate.go`, `run/execution.go`, `core/migrate.go`
- MUST update all review-parsing callers across the codebase: `reviews/store.go`, `run/execution.go`, `core/migrate.go`
- MUST move result/config types from `core` to `model`: `FetchResult`, `SyncConfig`, `SyncResult`, `ArchiveConfig`, `ArchiveResult`, `MigrationConfig`, `MigrationResult` (G2-F6, G2-F13)
- MUST update `kernel/handlers.go` `operations` interface to use `model.*` types instead of `core.*` types (G2-F6)
- MUST update `kernel/commands/*.go` to use `model` types instead of `core` for the relocated result/config shapes (G2-F13)
- MUST extract shared `resolveWorkflowTarget` helper from `sync.go`, `archive.go`, `migrate.go` (G2-F9, G2-F10)
- MUST extract shared task file walker into `tasks` package from `plan/input.go` and `tasks/store.go` (G4-F09)
- MUST fold `preputil` package into `plan` (move `ClosePreparationJournal` to `plan/`) (G4-F21)
- MUST resolve the `provider`/`providers` naming confusion either by renaming `internal/core/providers` to a clearer package name (for example `providerdefaults`) or by inlining `DefaultRegistry()` at its current call sites (G5-F09)
- MUST NOT change any behavior -- only code location and import paths
- MUST pass `make verify` with zero issues
</requirements>

## Subtasks

- [ ] 3.1 Move task parsing functions and sentinels from `prompt` to `tasks` package, update all callers
- [ ] 3.2 Move review parsing functions and sentinels from `prompt` to `reviews` package, update all callers
- [ ] 3.3 Clean up `prompt/common.go` -- only prompt-building functions should remain
- [ ] 3.4 Move result/config types from `core` to `model`, update `kernel/handlers.go` and `kernel/commands/*.go`
- [ ] 3.5 Extract shared `resolveWorkflowTarget` helper and shared task file walker
- [ ] 3.6 Fold `preputil` into `plan` and clarify the `providers` composition package naming/wiring

## Implementation Details

This is the highest-risk phase because it changes import paths across many packages. Execute subtasks in order: task parsing first (3.1), then review parsing (3.2), then prompt cleanup (3.3), then type relocation (3.4), then shared helpers (3.5), then cleanup (3.6). Run `make verify` after each subtask.

The `prompt` package should retain only: `Build`, `BuildSystemPromptAddendum`, `buildBatchHeader`, `buildBatchChecklist`, `FlattenAndSortIssues`, `SafeFileName`, `NormalizeForPrompt`, template loading, and `BatchParams`.

### Relevant Files

- `internal/core/prompt/common.go` — source of parsing functions to relocate (lines 52-175 for tasks, 272-369 for reviews)
- `internal/core/tasks/store.go` — destination for task parsing, already calls `prompt.ParseTaskFile`
- `internal/core/tasks/validate.go` — calls `prompt.ParseTaskFile`, `prompt.IsTaskCompleted`
- `internal/core/reviews/store.go` — destination for review parsing, already calls `prompt.ParseReviewContext`
- `internal/core/plan/input.go` — calls `prompt.ParseTaskFile` and contains duplicated parse-error wrapping logic
- `internal/core/plan/prepare.go` — calls `prompt.ParseTaskFile`
- `internal/core/run/execution.go` — calls `prompt.ParseTaskFile` and `prompt.ParseReviewContext`
- `internal/core/migrate.go` — calls both task/review parsing and the target-resolution helpers
- `internal/core/api.go` — defines `SyncConfig`, `ArchiveConfig`, `MigrationConfig`, etc. to move to model
- `internal/core/kernel/handlers.go` — `operations` interface uses `core.*` types (line 22-31)
- `internal/core/kernel/commands/*.go` — import `core` for config/result types
- `internal/core/sync.go` — `resolveSyncTarget` to extract shared helper from
- `internal/core/archive.go` — `resolveArchiveTarget` to extract shared helper from
- `internal/core/migrate.go` — `resolveMigrationTarget` to extract shared helper from
- `internal/core/preputil/journal.go` — single function to fold into `plan`
- `internal/core/providers/defaults.go` — candidate rename target or inlining source for `DefaultRegistry()`

### Dependent Files

- All test files in `tasks/`, `reviews/`, `plan/`, `run/`, `kernel/`, `core/` — imports will change
- `internal/cli/root.go` — may need import updates if the `providers` package is renamed instead of inlined
- `internal/core/workspace/config.go` — imports `providers`, needs update
- `internal/core/fetch.go` — imports `providers`, needs update

## Deliverables

- `prompt` package contains only prompt-building functions (no more domain parsing)
- `tasks` package owns all task parsing and error sentinels
- `reviews` package owns all review parsing and error sentinels
- `kernel` operations and command result/config types use `model.*` rather than the relocated `core.*` data shapes
- `preputil` package deleted, code in `plan`
- `provider`/`providers` naming or wiring clarified without changing behavior
- Shared `resolveWorkflowTarget` helper eliminates 3-way target resolution duplication
- `make verify` passes with zero issues **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] `tasks.ParseTaskFile` returns identical results to former `prompt.ParseTaskFile` for all test cases
  - [ ] `reviews.ParseReviewContext` returns identical results to former `prompt.ParseReviewContext` for all test cases
  - [ ] `tasks.WrapParseError` handles both `ErrLegacyTaskMetadata` and `ErrV1TaskMetadata`
  - [ ] `reviews.WrapParseError` handles `ErrLegacyReviewMetadata`
  - [ ] Shared `resolveWorkflowTarget` returns correct paths for sync, archive, and migrate configs
  - [ ] Shared task file walker produces identical results to `plan/input.readTaskEntries` and `tasks/store.countTasks`
  - [ ] All existing tests in `kernel/` pass with `model.*` types
  - [ ] All existing tests in `commands/` pass after removing `core` dependencies for the relocated result/config types
- Integration tests:
  - [ ] `make verify` passes (fmt + lint + test + build)
  - [ ] No `prompt.ParseTaskFile` or `prompt.ParseReviewContext` calls remain in the codebase
- All tests must pass

## Success Criteria

- All tests passing
- `make verify` exits 0
- `grep -r "prompt.ParseTaskFile\|prompt.ParseReviewContext" internal/` returns zero results
- `internal/core/kernel/handlers.go` no longer uses the moved `core.*` result/config types in the `operations` interface
- `internal/core/preputil/` directory no longer exists
- `provider`/`providers` naming confusion is resolved either by a clearer package name or by inlining `DefaultRegistry()` at the existing call sites
