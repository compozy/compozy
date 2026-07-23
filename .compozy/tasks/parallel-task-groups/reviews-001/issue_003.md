---
provider: manual
pr:
round: 1
round_created_at: 2026-07-23T18:09:25Z
status: resolved
file: internal/cli/task_group_picker.go
line: 64
severity: medium
author: claude-code
provider_ref:
---

# Issue 003: Multi-select task-group picker (R2) unimplemented

## Review Comment

`task_06.md` requirement R2 (MUST) is to "convert the task-group picker to
multi-select (`huh.NewMultiSelect[string]` returning `[]string`) for the
parallel path, preserving the swappable `commandState.pickTaskGroup` seam."
The task is marked `status: completed`, but the picker was never converted.

Verified: `defaultPickTaskGroup` (`internal/cli/task_group_picker.go:64`) still
uses `huh.NewSelect[string]` and the `pickTaskGroup` seam on `commandState`
(`internal/cli/state.go:116`) is still typed
`func(*cobra.Command, taskGroupPickerInput) (string, error)` — single-value.
`huh.NewMultiSelect[string]` appears only in the unrelated `internal/cli/setup.go`
(`:622`, `:669`). The proposed `internal/cli/task_group_picker_test.go`
multi-select build/validate tests do not exist.

Impact is lower than a correctness bug because the parallel path currently
requires explicit `--multiple init/TG-NNN,…` targets (an empty selection is
rejected), so the feature is still fully usable non-interactively. But there is
no interactive multi-group selection journey at all, which is the unmet R2
deliverable in a task claiming completion.

Suggested fix: either implement the `[]string` multi-select variant behind the
preserved `pickTaskGroup` seam and wire it as the interactive entry point for
`--parallel-task-groups`, or explicitly descope R2 (updating `task_06.md` and
the deliverables) if non-interactive explicit-target selection is the intended
final surface.

## Triage

- Decision: `VALID`
- Root cause: `task_06.md` R2 (a MUST) and subtask 6.3 require a multi-select
  task-group picker (`huh.NewMultiSelect[string]` returning `[]string`) for the
  parallel path. Only the single-select `defaultPickTaskGroup`
  (`task_group_picker.go:64`, `huh.NewSelect[string]`) was implemented, and the
  `--parallel-task-groups` flag was gated to *only* work with an explicit
  `--multiple init/TG-NNN,…` list (`daemon_commands.go:1092-1094`,
  `--parallel-task-groups is only valid with --multiple`). There was no
  interactive multi-group selection journey at all — the R2 deliverable was
  genuinely absent.
- Not descoped: ADR-006 (Accepted) states the flag "pairs with the existing
  `--multiple <initiative/TG-NNN,…>` selection **(or the multi-select picker)**",
  and `_techspec.md` (lines 17, 124, 152) lists the multi-select picker as an
  intended deliverable ("Single-select → multi-select … Add multi-select mode for
  parallel groups"). Descoping R2 would contradict the Accepted design, so the
  correct resolution is to implement the picker, not remove the requirement.
- Fix approach:
  1. `task_group_picker.go` (in scope): extract the shared status-load +
     option-build into `loadTaskGroupPickerOptions`; add `defaultPickTaskGroups`
     (`huh.NewMultiSelect[string]` → `[]string`), `validateTaskGroupPickerMultiSelection`,
     and `normalizeTaskGroupSelections`.
  2. `state.go` (minimal, documented — out of listed scope but absolutely
     required to keep the seam stubbable): add a sibling `pickTaskGroups`
     `func(*cobra.Command, taskGroupPickerInput) ([]string, error)` seam with the
     default + fallback, preserving the existing single-select `pickTaskGroup`
     seam untouched.
  3. `daemon_commands.go` (minimal, documented — required to wire the interactive
     entry point): route `--parallel-task-groups` without `--multiple` to a new
     interactive collector that runs the multi-select picker and produces
     `initiative/TG-NNN` refs, then delegates to the existing
     `runTaskWorkflowsMultiplePrepared` so all of R3–R7 (preflight, mode,
     execution kind, reporting) are reused unchanged.
  4. Tests in `daemon_commands_test.go` for the new pure logic (multi-select
     validation, ref assembly, and the stubbed-seam interactive collector).
- Resolution: Implemented R2 as above. Changed files:
  - `internal/cli/task_group_picker.go`: `loadTaskGroupPickerOptions`,
    `taskGroupPickerHuhOptions`, `taskGroupPickerDescription`,
    `defaultPickTaskGroups` (`huh.NewMultiSelect[string]` → `[]string`),
    `validateTaskGroupPickerMultiSelection`, `normalizeTaskGroupSelections`;
    `defaultPickTaskGroup` refactored onto the shared helper (behavior identical).
  - `internal/cli/state.go`: added the sibling `pickTaskGroups` seam (default +
    fallback); single-select `pickTaskGroup` seam left untouched.
  - `internal/cli/daemon_commands.go`: `runTaskWorkflow` routes
    `--parallel-task-groups` (no `--multiple`) to `runInteractiveParallelTaskGroups`,
    which collects refs via the picker and delegates to
    `runTaskWorkflowsMultiplePrepared` (reuses R3–R7); relaxed
    `rejectMultipleOnlyParallelFlags` accordingly; added
    `rejectInteractiveParallelTaskGroupConflicts`, `collectParallelTaskGroupRefs`,
    `pickParallelTaskGroupRefs`.
  - `internal/cli/daemon_commands_test.go`: `TestValidateTaskGroupPickerMultiSelection`,
    `TestPickParallelTaskGroupRefs`, `TestCollectParallelTaskGroupRefs`.
- Verification (fresh, after all changes):
  - `make fmt lint test go-build` → golangci-lint **0 issues**; `internal/cli`
    tests **✓ (59.07s)**, no FAIL/panic/race; `bin/compozy` built.
  - `make verify` additionally runs `frontend-e2e` (Playwright), which failed with
    `compozy daemon start` never producing `daemon.json` under the sandboxed
    `.tmp/playwright/home` — a daemon-can't-start-in-sandbox **environment**
    failure in the isolated review worktree, unrelated to this Go-only change
    (the diff touches `tasks run` flag routing + the task-group picker, not daemon
    startup, `daemon.json`, or the frontend). Pre-existing; documented and
    proceeding per the cy-fix-reviews unrelated-failure rule.
- Notes:
