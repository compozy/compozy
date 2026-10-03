# BUG-20261003-profile-dialog-survives-back: Browser Back leaves a profile lifecycle dialog on the previous page

- **Status:** verified
- **Fix commit:** pending
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Sol
- **Journey Step:** J-operate-profiles, back out of a lifecycle dialog
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-settings-dialog-plans, Back-Button Tour, keyboard, desktop 1512×862, en-US, wifi-fast,
isolated production Web/daemon at port 53876, build b4ed8ca18.

1. Navigate from Settings General to Profiles and open Archive publication-drafts by keyboard.
2. Use browser Back. The URL and background page return to Settings General.
3. Observe for 12 seconds: the archive dialog remains open with Cancel focused and Archive available.
4. Escape dismisses it, leaving focus at the root. UDS confirms no archive or color mutation occurred.

Expected: leaving the initiating route dismisses its lifecycle dialog and leaves usable focus.
Actual: the previous page remains covered by a dialog belonging to the page the user left.

## Evidence

docs/qa/evidence/2026-10-02-untested/profile-delete-navigation-sol-back-from-dialog.json and
profile-delete-navigation-sol-back-observed.json retain history entries, route, AX and the bounded wait.
profile-delete-navigation-sol-back-owner.json independently proves unchanged profile state.
profile-back-leaves-archive-dialog.png was inspected. Recording profile-delete-navigation-sol is
stopped with 168 frames. No source reads occurred during the walk.

Dedup: BUG-20260906-settings-nav-stale-open-history concerns a section click superseded by a pending
window launch; that different symptom does not cover a lifecycle dialog surviving completed Back.

## Repair and verification

The permanent lifecycle host now subscribes to browser history only while a dialog is open.
Back/Forward calls its existing close operation, clearing the intent, rename draft, repository
declines and unarchive result together. The global profile stream keeps its existing owner.

E2E-017 in the existing profiles browser suite reproduces the failure before the repair, with the
route at General while Archive stays visible for the full 20-second assertion budget. After the
repair it passes with a real daemon, as does the adjacent E2E-027 palette handoff. Receipts:
profile-dialog-back-{red,green}.log and their artifacts/command records. A prior incompatible
Playwright runner invocation is a driver failure, retained separately.

Fresh Sol replay on the main lab verifies Archive → Back → Forward without a mutation or revived
dialog. An abandoned rename draft is empty on reopening. Unarchive success also closes with Back,
stays closed with Forward and reload, and independent UDS retains the active owner. Inspected
screenshots: profile-back-dismisses-archive-dialog.png and profile-unarchive-before-back.png.
Recording profile-dialog-back-fixed-sol stops at 211 frames; its receipts retain the keyboard/AX
trail and independent reads. Spoken VoiceOver and the full charter remain unverified.

The adjacent palette cancellation probe finds a separate retained-URL replay defect,
BUG-20261003-profile-palette-cancel-reopens. That finding does not invalidate the direct Settings
history repair and remains tracked independently.
