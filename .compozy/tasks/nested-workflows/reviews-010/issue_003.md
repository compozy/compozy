---
provider: manual
pr:
round: 10
round_created_at: 2026-07-25T01:52:47Z
status: resolved
file: internal/cli/tasks_run_wizard.go
line: 570
severity: medium
author: claude-code
provider_ref:
---

# Issue 003: Malformed plans appear as runnable ordinary workflows

## Review Comment

`readTaskRunWizardPlan` returns the same `(Plan{}, false)` result when
`_task_groups.md` is absent, unreadable, empty, or malformed.
`buildTaskRunWizardWorkflowOptions` treats every false result as an ordinary
workflow and adds a selectable row based on root task files.

This contradicts `TargetResolver.ClassifyTarget`, whose contract explicitly
classifies a present invalid marker as `TargetModeInvalidOptIn` rather than an
ordinary workflow. The wizard can therefore advertise a malformed initiative
as runnable and display root-task progress, but dispatch later re-resolves the
same marker and rejects the selection with `ErrInvalidPlan`. A read-permission
or transient I/O error is hidden in the same way.

Return a typed plan-read result that distinguishes `os.ErrNotExist` from invalid
or unreadable markers. Keep the ordinary row only for a genuinely absent
marker; render present-invalid initiatives as blocked rows with the parse/read
diagnostic, matching the resolver's fail-closed classification. Add table-driven
wizard tests for missing, empty, malformed, and unreadable markers.

## Triage

- Decision: `VALID`
- Notes: `readTaskRunWizardPlan` collapses marker absence, `os.ReadFile`
  failures, and `taskgroups.ParsePlanForInitiative` failures into the same
  `false` result. `buildTaskRunWizardWorkflowOptions` then converts every such
  result into a selectable ordinary-workflow row, even though the execution
  resolver fails closed for a present unreadable or invalid marker. The fix
  returns an explicit absent/valid/invalid read state, preserves the read/parse
  diagnostic, and creates a non-selectable blocked row for invalid opt-ins. The
  builder lives in `internal/cli/tasks_run_wizard_status.go`, so that file
  required a minimal scope extension to consume the typed result and render the
  diagnostic; regression coverage remains in the existing wizard test suite.
- Verification: The focused race-enabled regression passes (`go test -race
  ./internal/cli -run 'TestTaskRunWizardModel/Should_fail_closed' -count=1`: 6
  passed). Full `make verify` is blocked by a pre-existing, out-of-scope lint
  finding in unchanged `internal/cli/daemon_commands.go:1958`
  (`resolveTaskPresentationMode` complexity 16 exceeds 15). An independent
  package run also reproduced eight unchanged daemon integration setup failures
  because their temporary `daemon.json` files were never created. Per the clean
  verification gate, this issue remains `valid` and no commit is created.
