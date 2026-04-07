---
status: completed
title: "Phase 0: Trivial quick wins"
type: refactor
complexity: medium
dependencies: []
---

# Task 01: Phase 0: Trivial quick wins

## Overview

Eliminate 15 zero-risk DRY violations, dead code, and coding style issues across the codebase. These are mechanical fixes that reduce noise and establish a cleaner baseline for the structural refactoring phases that follow. Every item is independently safe to apply and requires no API or import path changes.

<critical>
- ALWAYS READ the TechSpec (20260406-summary.md) and detailed reports before starting
- REFERENCE the individual analysis reports (20260406-*.md) for exact file:line locations
- FOCUS ON "WHAT" — each subtask is a specific, localized fix
- MINIMIZE CODE — no new abstractions, just deduplication and cleanup
- TESTS REQUIRED — run `make verify` after each group of changes
</critical>

<requirements>
- MUST delete duplicate `newPreparation`/`newJob` in `kernel/handlers.go` and reuse the `core` package version (G2-F1)
- MUST consolidate `wrapTaskParseError` into a single location in `tasks` package, update `plan/input.go` to call it (G4-F02)
- MUST consolidate `wrapReviewParseError` into a single location in `reviews` package, update `plan/input.go` to call it (G4-F03)
- MUST hoist 9+ `regexp.MustCompile` calls from function bodies to package-level `var` declarations in `prompt/common.go` and `plan/input.go` (G4-F07)
- MUST replace `reflect.DeepEqual` with `slices.Equal` in `cli/root.go:1027` (G1-F10)
- MUST remove duplicate `clampInt` in `run/validation_form.go`, use existing `clamp` from `ui_layout.go` (G3-F14)
- MUST consolidate `copyJSON`/`copyJSONPayload` into a single function in `run/` (G3-F24)
- MUST remove unused parameters from `notifyJobStart` in `run/command_io.go` (G3-F18)
- MUST wire `internal/version` import in `cmd/compozy/main.go` or document linker-only usage (G5-F02)
- MUST fix `mustReadTemplate` in `prompt/templates.go` to `panic` on missing embedded template instead of returning empty string (G4-F20)
- MUST replace `fmt.Println` calls with `slog.Info` in `plan/input.go` (G4-F08)
- MUST add `job.codeFileLabel()` method to `run/types.go` and replace 9+ `strings.Join(*.codeFiles, ", ")` occurrences (G3-F13)
- MUST extract `ShutdownBase` embedded struct in `events/kinds/shutdown.go` (G5-F05)
- MUST extract `JobAttemptInfo` embedded struct in `events/kinds/job.go` (G5-F06)
- MUST merge `groupIssues` into `groupIssuesByCodeFile` in `plan/` (G4-F11)
- MUST pass `make verify` with zero lint issues after all changes
</requirements>

## Subtasks

- [x] 1.1 Eliminate cross-package duplicate functions (`newPreparation`/`newJob`, `wrapTaskParseError`, `wrapReviewParseError`, `clampInt`, `copyJSON`, `groupIssues`)
- [x] 1.2 Hoist `regexp.MustCompile` calls to package-level vars in `prompt/common.go` and `plan/input.go`
- [x] 1.3 Fix coding style violations (`reflect.DeepEqual` -> `slices.Equal`, `fmt.Println` -> `slog.Info`, unused params, `mustReadTemplate` panic)
- [x] 1.4 Add `job.codeFileLabel()` method and replace scattered `strings.Join` calls
- [x] 1.5 Extract shared embedded structs in event payloads (`ShutdownBase`, `JobAttemptInfo`)
- [x] 1.6 Wire `internal/version` import or document linker-only usage
- [x] 1.7 Run `make verify` and fix any issues

## Implementation Details

All changes are localized within existing packages. No import path changes, no new packages, no API changes. See the TechSpec Phase 0 table for the exact file locations and findings.

### Relevant Files

- `internal/core/kernel/handlers.go` — duplicate `newPreparation`/`newJob` to delete (lines 258-289)
- `internal/core/api.go` — canonical `newPreparation`/`newJob` to keep (lines 393-427)
- `internal/core/plan/input.go` — `wrapTaskParseError`, `wrapReviewParseError`, `groupIssues`, `fmt.Println`, regexes
- `internal/core/tasks/store.go` — `wrapTaskParseError` duplicate to consolidate
- `internal/core/reviews/store.go` — `wrapReviewParseError` duplicate to consolidate
- `internal/core/prompt/common.go` — 9+ `regexp.MustCompile` inside function bodies to hoist
- `internal/cli/root.go` — `reflect.DeepEqual` at line 1027
- `internal/core/run/validation_form.go` — `clampInt` duplicate at line 225
- `internal/core/run/ui_layout.go` — canonical `clamp` at line 45
- `internal/core/run/events.go` — `copyJSON` to consolidate
- `internal/core/run/session_view_model.go` — `copyJSONPayload` duplicate
- `internal/core/run/command_io.go` — `notifyJobStart` with unused params (line 47)
- `internal/core/run/types.go` — add `codeFileLabel()` method
- `internal/core/run/execution.go` — 9+ `strings.Join(*.codeFiles, ", ")` call sites
- `internal/core/prompt/templates.go` — `mustReadTemplate` error handling
- `cmd/compozy/main.go` — wire `internal/version` import
- `pkg/compozy/events/kinds/shutdown.go` — extract `ShutdownBase`
- `pkg/compozy/events/kinds/job.go` — extract `JobAttemptInfo`
- `internal/core/plan/prepare.go` — `groupIssuesByCodeFile` to keep

### Dependent Files

- `internal/core/run/execution.go` — affected by `codeFileLabel()` refactor (9+ call sites)
- `internal/core/run/command_io.go` — affected by `codeFileLabel()` refactor
- `internal/core/plan/input.go` — affected by consolidating error wrappers and groupIssues removal
- Test files for all modified packages — must still pass

## Deliverables

- All 15 quick-win items applied
- Zero duplicate functions remaining for the listed items
- All regexes at package-level vars
- `make verify` passes with zero issues **(REQUIRED)**

## Tests

- Unit tests:
  - [x] Existing tests for `kernel/handlers.go` still pass after `newPreparation`/`newJob` deletion
  - [x] Existing tests for `tasks/store.go` still pass after `wrapTaskParseError` becomes the single source
  - [x] Existing tests for `reviews/store.go` still pass after `wrapReviewParseError` becomes the single source
  - [x] `clamp` function handles all cases previously covered by `clampInt`
  - [x] `job.codeFileLabel()` returns the same output as inline `strings.Join` calls
  - [x] `ShutdownBase` embedding preserves JSON serialization compatibility
  - [x] `JobAttemptInfo` embedding preserves JSON serialization compatibility
- Integration tests:
  - [x] `make verify` passes (fmt + lint + test + build)
- All tests must pass

## Success Criteria

- All tests passing
- `make verify` exits 0
- Zero duplicate functions for the 6 identified duplication pairs
- Zero `regexp.MustCompile` calls inside function bodies in `prompt/common.go` and `plan/input.go`
- Zero `reflect.DeepEqual` usage in `cli/root.go`
- Zero `fmt.Println` calls in `plan/input.go`
