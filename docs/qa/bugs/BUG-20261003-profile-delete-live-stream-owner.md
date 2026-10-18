# BUG-20261003-profile-delete-live-stream-owner: Deleting the viewed profile disables its own recovery stream

- **Status:** verified
- **Fix commit:** fb4b8a40a
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

## Diagnosis and repair

The archive recorder repair passes independently. During delete, the profile lifecycle host used
the desktop continuity-stream budget, which disabled its global recovery feed while the deleted
profile's window-manager authority reconnected. The shell now keeps the profile feed mounted for
the document's lifetime, independent of that desktop budget. Other continuity consumers retain
their existing budget. No polling, reload, server ownership relaxation or wire change is added.

Invariant, owner, canonical suite: E2E-030 in web/e2e/__tests__/profiles.spec.ts uses a real daemon
to require external deletion of the viewed profile to recover the browser to default without
reload, remove that profile from the picker, and retain a neighboring profile. Before repair the
default assertion times out while the deleted name remains visible. The regression and adjacent
E2E-013/E2E-014 pass after rebuilding the production Web bundle. React Doctor remains 100/100.

## Fresh public replay

Dora selects a fresh delivery-drafts profile and opens Home, then deletes it through the CLI.
The existing global socket receives profile.deleted while the desktop reconnects; the browser
returns to default without reload. UDS finds the durable event with the deleted subject and a
profile read returns 404. The original five profiles remain unchanged. Reload retains default,
the picker excludes the deleted name, and the browser is restored to studio.

Receipts: profile-delete-stream-red-behavior.log, profile-delete-stream-red-artifacts/,
profile-delete-stream-green.log, profile-delete-stream-build.log,
profile-delete-react-doctor-{baseline,after}.log and profile-delete-fixed-*.json in this cycle's
evidence directory. Screenshot: profile-delete-live-recovered.png. Recording:
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/profile-delete-fixed (9 frames,
stopped). No source or database reads occurred during the replay. Earlier CLI-version and Home
selector setup failures are retained separately and are not counted as product regressions.

The complete Settings/switcher charter remains Pending; this verifies the lifecycle recovery fix.

Delivery gate: all affected make gate lanes passed, including root-Turborepo Web validation
(693 files, 6,903 tests). Receipt: profile-delete-stream-gate.log.
