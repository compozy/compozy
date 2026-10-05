# BUG-20261004-task-execution-switches-unnamed: Task execution switches have no accessible name

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea (discovery); keyboard and assistive-technology users
- **Journey Step:** J-scope-global-across-workspaces, create a saved draft
- **Scenarios:** MS-global-scope-no-workspace-work
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The task form visually offers Save as draft and Start automatically when ready, but the browser
accessibility tree exposes both switches without a name. Their states cannot be identified
reliably through assistive technology. The visible labels are present.

## Reproduction

- **Charter:** CH-profile-global-phase-zero · **Tour:** Feature Tour
- **Environment:** real Chrome, 1512 × 862, Wi-Fi, en-US; isolated daemon at port 50727.

1. Open Tasks and New task while Global is active.
2. Choose Advanced and expand Execution.
3. Inspect the two switches through the browser accessibility tree.

Expected: each switch has the same accessible name as its visible label.
Actual: both names are empty, even though their checked states are exposed.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/:
- global-work-lea-draft-controls.json records the rendered controls and their neighboring labels.
- global-work-lea-execution-options.png shows the visible labels.
- global-work-lea-draft-ready.json records both empty accessible names.
- global-work-lea-ended.json closes the original persona before source diagnosis.
- Recording: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/global-work-lea.

No spoken screen-reader result is claimed.

## Fix

- Root cause: ExecutionCollapsible renders FieldTitle next to each Switch without an accessible
  labeling relationship.
- Repair: connect each existing visible FieldTitle through a unique aria-labelledby target.
  No new primitive, copy, timer, persistence, or execution policy.
- Fix commit: pending.
- Regression test: the existing task-editor-modal.test.tsx execution-options case now locates both
  switches by role and accessible name and verifies each corresponding draft mutation. The first
  run fails on the missing Save as draft name; all 19 owning tests pass after the production fix.

## Verification

Fresh Lea replay passes with both switches named in the real Chrome accessibility tree. Lea
creates a Global draft after toggling the controls; CLI, HTTP and UDS confirm the saved draft
values, and the same item survives a reload and daemon restart. The adjacent Studio project
form exposes both names and accepts pointer and Space activation; cancelling creates no task.

The existing 19-test editor suite, root Turbo lint/typecheck/build, and React Doctor (100/100)
pass. Receipts use global-work-execution-switches- and global-work-fixed-lea- under the evidence
directory. The 63-frame global-work-fixed-lea recording is closed. Both owned drafts, both
stopped sessions and the temporary user resource are removed through public interfaces; the
original workspace catalog, profile selections and four profile sessions remain unchanged.

Delivery commit remains pending. A separate Global Agents catalog finding keeps the broader
scenario open; it does not invalidate this control-label replay. No spoken screen-reader result
is claimed.
