# BUG-20261003-profile-create-stale-name-error: Editing a refused profile name leaves the old refusal on the new value

- **Status:** verified
- **Fix commit:** 8380b94f2
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Sol
- **Journey Step:** J-operate-profiles, correct a refused profile name
- **Scenarios:** ET-profile-switcher-restore; ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Create from the keyboard and submit default. After its inline reserved-name refusal, replace it
with release-drafts. The old reserved-name message and aria-invalid=true remain attached to that
new value. Choose a notebook-pen icon and a valid Blue color, then submit: the daemon accepts
release-drafts. Independent UDS/HTTP reads and a browser reload retain the created profile,
proving that the old message described a different input. The duplicate-toast repair passes
independently; this is a separate stale-field defect.

## Evidence

Cycle receipts: profile-dialogs-sol-fixed-correct-and-search.json, profile-dialogs-sol-fixed-picker.json,
profile-dialogs-sol-fixed-created.json, profile-dialogs-sol-fixed-created-read.json,
profile-dialogs-sol-fixed-persisted.json and profile-dialogs-sol-fixed-ended.json. Screenshot
profile-dialogs-picker-invalid-color.png also preserves the stale name refusal beside the changed
name. Recording profile-dialogs-sol-fixed is stopped (95 frames). The owned profile is retained
for the same lab's continuing Settings walk. No source or database reads occurred during the walk.

## Owning regression

ProfileCreateDialog's existing component suite owns whether its input is currently invalid.
Extend the blank-name case and add a server-refusal case: changing the name clears the obsolete
field error, returning to the refused name retains it, and submitting a corrected name still
calls the creation boundary. The daemon remains the validation owner; do not reset a live
mutation or discard its error to repair field presentation.

## Fix and fresh replay

The dialog previously rendered the mutation's error string against every new input value, and
retained the local blank-name error after typing. Name-specific daemon refusals now carry their
submitted name from the mutation's variables. The field derives validity from the current trimmed
value; returning to the rejected value restores its refusal. Other operation failures remain
visible at form level. Editing does not reset the mutation or change its pending state.

The existing ProfileCreateDialog suite failed both stale-field cases before the repair. All six
cases pass afterwards, including operation-error retention and pending-submission protection.
The Storybook example follows the same internal prop contract. Root Turbo build/typecheck passes;
React Doctor reports 100/100 over 38 changed files, without suppression. The initial typecheck
caught an unsupported test-query option, corrected without weakening assertions. Moving error
classification into the existing lifecycle error helper keeps the dialog host within its
complexity budget. Receipts retain both intermediate diagnostics and their successful reruns.

Sol re-enters through the profile switcher using only the keyboard. Blank-name feedback clears
on typing, a reserved-name refusal clears after correction and returns when default is re-entered,
and reading-room is created with its starter identity. UDS confirms identity
01M40J0V0Z7DETPHF3VTRS9464; reload retains the profile and selection. A driver select-all gesture
failed on its second use, leaving reading-roodefault; the captured input proves that timeout was
not a product failure. Ordinary Backspace keys restored the intended input before the replay
continued. Emoji customization failed independently and has its own open finding.

Regression test: web/src/systems/profiles/components/__tests__/profile-create-dialog.test.tsx.
Receipts: profile-create-name-error-{red,green}.log, profile-create-name-error-build-final.log,
profile-create-name-error-react-doctor-final.log, profile-create-name-error-sol-*.json and
profile-create-feedback-e2e-final.log. Screenshots: profile-create-corrected-name.png and
profile-reading-room-persisted.png. Recording profile-create-name-error-sol is stopped (138 frames).
No source or database reads occurred during the replay. Fix commit 8380b94f2 follows current-pass delivery gates; receipts are
profile-create-feedback-gate-final.log, profile-create-feedback-gate-status.log and
profile-create-feedback-commit.log.
