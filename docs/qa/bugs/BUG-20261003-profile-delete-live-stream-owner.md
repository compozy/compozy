# BUG-20261003-profile-delete-live-stream-owner: Deleting the viewed profile disables its own recovery stream

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, retire an empty profile while its browser stays open
- **Scenarios:** ET-profile-switcher-restore; ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Select the owned delivery-archive-check profile in the isolated browser. Confirm its profile log
WebSocket is connected (101). Delete that empty profile from the public CLI. Deletion succeeds and
an independent UDS log read finds profile.deleted under the permanent operator, retaining the deleted
subject in content. The browser closes its sockets, retries unavailable window-manager authority,
and keeps the deleted profile visible. Waiting for default fails. Switching manually to studio
recovers. An earlier release-drafts deletion showed the same symptom.

## Evidence

Cycle receipts: profile-archive-fixed-select-delete.json, profile-archive-fixed-delete-active.json,
profile-archive-fixed-deleted-view.json, profile-archive-fixed-delete-audit.json,
profile-archive-fixed-delete-stale-recover.json, profile-archive-fixed-ended.json;
profile-delete-live-stale.png. Recording profile-archive-fixed is stopped. The retained temporary
profiles have been removed and both browser and CLI are restored to studio.

## Investigation

The archive recorder repair passes independently. During delete, the profile lifecycle host uses
the desktop continuity-stream budget, which disables its global recovery feed while the deleted
profile's window-manager authority reconnects. Inspect the owning composition and add regression
coverage before changing that lifetime. No production edits for this defect yet.
