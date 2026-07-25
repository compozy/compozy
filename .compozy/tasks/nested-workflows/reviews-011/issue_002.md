---
provider: manual
pr:
round: 11
round_created_at: 2026-07-25T03:30:01Z
status: resolved
file: internal/cli/tasks_run_wizard.go
line: 599
severity: medium
author: claude-code
provider_ref:
---

# Issue 002: Wizard accepts task-group markers outside the initiative

## Review Comment

`readTaskRunWizardPlan` uses `os.Lstat` only to distinguish a missing marker,
then reads `_task_groups.md` with `os.ReadFile`. If the marker is a symlink,
`ReadFile` follows it. A symlink to a valid plan outside the initiative is
therefore classified as `taskRunWizardPlanValid`, and
`buildTaskRunWizardWorkflowOptions` renders its task groups as selectable run
targets.

Runtime resolution does not accept the same target:
`taskgroups.resolveMarker` resolves the symlink and requires the result to stay
inside the initiative. Dispatch consequently fails with `ErrContainment` after
the wizard advertised and accepted the target. This leaves the new fail-closed
invalid-plan UI inconsistent with the authoritative resolver.

Build picker options through `taskgroups.TargetResolver` or apply the same
`EvalSymlinks` and containment validation before reading the marker. A present
marker that resolves outside the initiative must become an unavailable row
with the containment diagnostic, not a valid group. Add a wizard regression
whose `_task_groups.md` symlinks to a valid external plan and assert that the
initiative is visible but locked.

## Triage

- Decision: `VALID`
- Notes: `readTaskRunWizardPlan` performs its own `Lstat`/`ReadFile`/parse sequence instead of applying the authoritative resolver's symlink-containment rule. `os.ReadFile` follows a present marker symlink, so a valid plan outside the initiative is exposed as selectable even though runtime resolution rejects the same path with `taskgroups.ErrContainment`. The fix resolves both the initiative and marker paths with `filepath.EvalSymlinks`, rejects a resolved marker outside the initiative before reading it, and preserves `ErrContainment` as the locked-row diagnostic. The existing fail-closed wizard table gains an external-marker-symlink regression that verifies the initiative remains visible, blocked, and unselectable with the containment error.
