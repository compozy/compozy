---
status: pending
domain: CLI & TUI
type: Feature Implementation
scope: Full
complexity: medium
dependencies:
  - task_02
---

# Task 4: compozy start Preflight + Bubble Tea Validation Form

## Overview

Wire the validator into `compozy start` as a preflight gate: when invalid task metadata is detected in TTY mode, present a Bubble Tea modal offering Continue / Abort / Copy fix prompt; in non-TTY mode, print the fix prompt to stderr and exit 1 unless `--force` is passed. This gate prevents wasted agent runs on bad metadata and gives the user a frictionless path to recovery.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST run `tasks.Validate` against the resolved tasks directory before any jobs are created in `compozy start`
- MUST skip the preflight entirely when `--skip-validation` is passed
- MUST present a Bubble Tea modal in TTY mode when the report is not OK; modal must offer exactly three actions: Continue anyway, Abort, Copy fix prompt
- MUST write the fix prompt to stderr (not stdout) when the user picks "Copy fix prompt" (clipboard access is out of scope — we print so users can pipe/select)
- MUST exit with code 0 only when validation passes OR the user picked "Continue anyway" OR `--force` was set
- MUST exit with code 1 when the user picks Abort, when non-TTY is detected without `--force`, or when `--force` is absent in a non-TTY environment
- MUST detect TTY via the existing `isInteractive` callback pattern used in `internal/cli/root.go` (line 54)
- MUST log the user's preflight decision via `slog` (ok | continued | aborted | forced | skipped) so CI post-mortems can see it
- SHOULD render the modal using existing `ui_styles.go` constants for visual consistency with the main TUI
</requirements>

## Subtasks
- [ ] 4.1 Create `internal/core/run/validation_form.go` with a Bubble Tea model (state, update, view) exposing the three actions and the fix-prompt text.
- [ ] 4.2 Add a `PreflightCheck(ctx, tasksDir, registry, isInteractive, force) (PreflightDecision, error)` entry-point (in `internal/core/run/` or a new `internal/core/run/preflight.go`) that runs `tasks.Validate` and dispatches to the modal or non-TTY path.
- [ ] 4.3 Add `--skip-validation` and `--force` flags to `newStartCommand()` in `internal/cli/root.go` (lines 154-174).
- [ ] 4.4 Call the preflight hook from the start command before `core.Run(...)` (line 280 area); short-circuit on abort.
- [ ] 4.5 Add structured `slog` log entries for each preflight outcome.
- [ ] 4.6 Add Bubble Tea unit tests (model update/view) and an integration test for the non-TTY path.

## Implementation Details

The modal component lives as a standalone Bubble Tea model (`tea.Model` with `Init`, `Update`, `View`); it is NOT merged into the main `uiModel`. It is a short-lived, blocking program started only when the report has issues. Reuse Lipgloss styles from `internal/core/run/ui_styles.go` for borders, colors, and highlights.

`PreflightCheck` is the clean seam between CLI and TUI: it takes a `*tasks.TypeRegistry` (built from the resolved `workspace.ProjectConfig`), calls `tasks.Validate`, decides the path, returns a typed `PreflightDecision` (one of `PreflightOK`, `PreflightContinued`, `PreflightAborted`, `PreflightSkipped`, `PreflightForced`). The CLI maps the decision to exit codes.

In non-TTY mode, print a concise summary + the fix prompt to stderr and return `PreflightAborted` (unless `--force`). The CLI converts `PreflightAborted` → exit 1.

Refer to TechSpec "API Endpoints" for the flag contract and to ADR-003 for the modal actions.

### Relevant Files
- `internal/core/run/validation_form.go` — NEW, Bubble Tea modal.
- `internal/core/run/validation_form_test.go` — NEW, model update/view tests.
- `internal/core/run/preflight.go` — NEW, `PreflightCheck` entry-point (or inline in an existing run/ file).
- `internal/cli/root.go` (lines 27-61, 154-174, 280) — add flags, call preflight before `core.Run`.
- `internal/core/run/ui_styles.go` (lines 27-29) — reuse color constants.

### Dependent Files
- `internal/core/tasks/validate.go` / `fix_prompt.go` (task_02) — imported and called.
- `internal/core/workspace/config.go` — `workspace.Resolve()` provides the config used to build `TypeRegistry`.
- `internal/core/run/ui_model.go` — no change expected, but the validation form must not interfere with the main `uiModel` (run as a separate short-lived Tea program).

### Related ADRs
- [ADR-003: Validation Command Architecture](adrs/adr-003.md) — Preflight UX contract: modal actions, TTY/non-TTY behavior, `--force`/`--skip-validation` semantics.

## Deliverables
- Bubble Tea validation form component.
- `PreflightCheck` entry-point.
- `--skip-validation` and `--force` flags added to `compozy start`.
- Structured slog logging of preflight decisions.
- Unit tests with 80%+ coverage **(REQUIRED)**.
- Integration test for the non-TTY preflight path **(REQUIRED)**.

## Tests
- Unit tests:
  - [ ] Model `Update` on key `c` transitions state to "continued" and quits with `PreflightContinued`.
  - [ ] Model `Update` on key `a` or `esc` quits with `PreflightAborted`.
  - [ ] Model `Update` on key `p` writes the fix prompt to stderr and quits.
  - [ ] Model `View` renders the list of offending files and issues from the supplied Report.
  - [ ] `PreflightCheck` with `skipValidation=true` returns `PreflightSkipped` without calling `tasks.Validate`.
  - [ ] `PreflightCheck` with a clean report returns `PreflightOK`.
  - [ ] `PreflightCheck` in non-TTY with `force=false` and issues returns `PreflightAborted` and writes fix prompt to stderr.
  - [ ] `PreflightCheck` in non-TTY with `force=true` and issues returns `PreflightForced`.
- Integration tests:
  - [ ] Run `compozy start --tasks-dir <invalid-fixtures> --skip-validation` — skips validation, attempts job setup (assert by log line or a fake runner).
  - [ ] Run `compozy start --tasks-dir <invalid-fixtures>` in non-TTY → exit code 1, stderr contains fix prompt.
  - [ ] Run `compozy start --tasks-dir <invalid-fixtures> --force` in non-TTY → continues past preflight, logs `preflight=forced`.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build with zero issues)
- `compozy start --help` documents `--skip-validation` and `--force`
- Bubble Tea modal renders correctly at terminal widths from 60 to 200 columns (manual smoke-test acceptable)
