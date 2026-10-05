# BUG-20261004-help-tip-discards-draft: Dismissing a hovered help tip discards the task draft

- **Status:** open
- **Impact (user-side):** Data-Loss
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, read field guidance while composing a task
- **Scenarios:** MS-web-modal-help-tips; MS-web-entity-modal-shell
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Escape dismisses both a hovered help tip and the Create task dialog, losing the unsaved title.
Reopening the dialog shows an empty form. Opening the same tip with keyboard focus behaves
correctly: Escape dismisses only the tip and leaves the dialog open.

## Reproduction

- **Charter:** CH-untested-041-administer-runtime-settings-dora · **Tour:** Back-Button Tour
- **Environment:** real isolated daemon, Chrome 1512 × 862, en-US, resume-editorial profile,
  Studio Operations workspace; final Settings shortcut bundle, source b4166a6c2

1. Open Tasks through the application and choose New task.
2. Type `Modal tour draft 2026-10-04` in Title.
3. Hover About what needs doing until its explanatory sentence appears, leaving focus in Title.
4. Press Escape once. Both the tip and the Create task dialog disappear, without confirmation.
5. Choose New task again. Title is empty, and a CLI task query finds no saved task with that title.
6. Tab to About what needs doing. The focused trigger exposes the same sentence; Escape now
   dismisses the tip while preserving the dialog and trigger focus.

**Expected:** The first Escape closes the open tip and preserves the enclosing form and its input.
**Actual:** The hover path also closes the form and destroys its unsaved input.

## Evidence

Receipts under docs/qa/evidence/2026-10-02-untested/:

- shared-modals-dora-draft-escape.json and shared-modals-dora-draft-help.png: populated title,
  visible help, physical Escape and no remaining dialog.
- shared-modals-dora-draft-escape.png and shared-modals-dora-reopened-draft.png: closed form
  followed by an empty reopened title.
- shared-modals-dora-draft-reopen-keyboard.json: empty title and the working keyboard-focus case.
- shared-modals-dora-task-before.json / shared-modals-dora-task-after.json: independent CLI
  queries both return zero matching tasks.
- shared-modals-dora-ended.json: Cancel restores Tasks; the nine-frame recording is closed.

All screenshot checkpoints were inspected. The initial hover probe incorrectly expected a
`tooltip` accessibility role; the actual visible guidance has StaticText. That driver assertion
is retained separately and is not evidence of missing guidance.

## Fix

- **Root cause:** A hovered tooltip leaves focus in the input. Base UI 1.7.0 handles Escape
  on the enclosing dialog's React event path before the tooltip's document listener. The two
  composed roots do not coordinate that dismissal, so the dialog unmounts the form first.
  Focus/click works because the tooltip trigger receives and consumes the key first.
- **Fix commit:** Pending.
- **Regression test:** packages/ui/src/components/custom/__tests__/help-tip.test.tsx. The existing
  suite composes the real Dialog and HelpTip, enters a title, opens guidance by hover/focus/click,
  and requires the first Escape to preserve the dialog, value and focus. A second Escape closes
  it. The hover case fails before repair while focus/click pass (38 pass / one fail total).

The shared tooltip registers its open lifetime with its nearest shared dialog. That dialog
cancels only its own Escape dismissal and allows propagation, using Base UI's public change-event
API; the tooltip's existing listener then closes the tip normally. Closed/disabled/unmounted tips
do not reserve dismissal, and peer dialogs retain independent ownership. No document capture
handler, dependency patch, focus transfer, task-specific exception or persistence change is added.

The official Tooltip event API documents `cancel` and `allowPropagation`:
https://base-ui.com/react/components/tooltip#root. The inspected 1.8.0 release and its hover/focus
timing fix do not address this composition ordering; no unrelated dependency upgrade is included.

## Successful original-persona replay

Dora's fresh help-tip-touch-dora session passes against source hashes recorded in
help-tip-touch-pointer-build-identity.json. Task hover/focus/click and repeated narrow touch
retain readable guidance and preserve the form's input on first Escape. Outside touch dismisses
only the tip; second Escape explicitly closes the form. The Job canary retains name, prompt and
caret, with continued typing. No drafts are saved, confirmed by independent CLI queries.
All five screenshots are inspected and the 20-frame recording is closed. Evidence:
help-tip-touch-dora-task-paths.json, help-tip-touch-dora-job-ended.json and the paired
help-tip-touch-dora-*-after.json receipts. Gate completion and commit linkage remain pending.
