# BUG-20261003-profile-dialog-validation-toast: Profile validation duplicates inline feedback in a technical toast

- **Status:** verified
- **Fix commit:** 8380b94f2
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Sol
- **Journey Step:** J-operate-profiles, create a profile through the keyboard
- **Scenarios:** ET-profile-switcher-restore; ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

In the fresh profile-dialogs lab, reach the quiet Profile trigger by Tab, open it with Enter,
and navigate to Create profile with the arrow keys. Enter the already-visible default name,
Tab to Create and press Enter. The name field becomes invalid and shows its inline refusal,
but a separate toast also exposes the raw daemon message, profile name "default" is reserved.
This duplicates the feedback the dialog already owns. Escape returns focus to Profile; an
independent UDS catalog confirms that no partial profile was created.

## Evidence

Cycle receipts: profile-dialogs-sol-name-taken.json, profile-dialogs-sol-cancel-end.json,
profile-dialogs-sol-no-partial-profile.json and profile-dialogs-sol-ended.json. Screenshot:
profile-dialogs-name-refusal.png. Recording profile-dialogs-sol is stopped (27 frames).
All interaction used the keyboard. Accessibility-tree state was observed; spoken VoiceOver
announcements were not verified. No source or database reads occurred during the walk.

## Investigation

After the walk, use-profile-mutations.ts shows that every lifecycle mutation calls reportFailure,
which emits a toast. Their sole production consumer, useProfileLifecycleDialogs, also passes
each mutation's error to its owning dialog. The stale-plan callbacks independently reread the plan.
Remove the duplicate notification owner while preserving inline errors, failed mutation state,
plan recovery and success feedback. No error should become silent.

Invariant, owner, canonical suite: E2E-014 in web/e2e/__tests__/profiles.spec.ts owns the Settings
create flow. Extend it with real-daemon reserved and duplicate-name refusals, requiring an invalid
name field and visible inline alert with no toast, followed by successful corrected creation.

## Fix and fresh replay

Removed reportFailure and the six lifecycle mutation onError notifications. Their existing dialog
consumer remains the feedback owner; mutation errors, success notices and stale-plan rereads are
preserved. The real-daemon E2E-014 fails before the repair with one unexpected toast and passes
afterwards with reserved and duplicate-name refusals. Adjacent E2E-016 and E2E-017 also pass.

Sol's fresh keyboard replay shows only the inline reserved-name refusal. Corrected creation,
independent UDS identity reads and reload all pass. The replay discovered a separate stale-name
error, subsequently repaired and re-walked, and a separate emoji arrow-navigation finding, still open.

Regression test: web/e2e/__tests__/profiles.spec.ts, E2E-014.
Receipts: profile-dialog-validation-{red,green}.log, profile-dialog-validation-red-artifacts/,
profile-dialogs-sol-fixed-*.json, profile-create-name-error-sol-*.json and
profile-create-feedback-e2e-final.log in this cycle's evidence directory. The final adjacent
E2E run passed all three cases in 27.4 seconds. Fix commit 8380b94f2 follows current-pass delivery gates; receipts are
profile-create-feedback-gate-final.log, profile-create-feedback-gate-status.log and
profile-create-feedback-commit.log.
