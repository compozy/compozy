# BUG-20261003-profile-palette-cancel-reopens: A canceled palette lifecycle dialog reopens on reload

- **Status:** verified
- **Fix commit:** pending
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Sol
- **Journey Step:** J-operate-profiles, cancel a palette lifecycle action
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-settings-dialog-plans, keyboard, desktop 1512×862, en-US, wifi-fast,
production Web/daemon at port 50727, b4ed8ca18 plus ownership and Browser Back repairs.

1. Open the command palette, choose Archive profile and supply navigation-notes.
2. The canonical Archive navigation-notes dialog opens; UDS still reports the profile active.
3. Escape dismisses Archive and returns to the palette. Dismiss the palette as well.
4. Reload. The URL still carries flow=archive&profile=navigation-notes and Archive opens again.

Expected: canceling a lifecycle action leaves it canceled after reload.
Actual: the retained route intent launches the same dialog again. No archive mutation occurred.

## Evidence

In docs/qa/evidence/2026-10-02-untested/: profile-dialog-back-fixed-sol-palette-archive-run.json
shows the canonical handoff, palette-archive-after and final-owner independently prove active state,
and profile-dialog-back-fixed-sol-final-state.json records the retained URL and reopened dialog.
The profile-dialog-back-fixed-sol recording stopped at 211 frames. Source was not read during
the walk. Earlier driver label/focus assumptions are retained separately from this observed failure.

Dedup: the Browser Back finding concerns an open dialog surviving a route change. This finding
concerns a canceled action being replayed from persisted window search on reload.

## Investigation

useProfileFlowIntent raises the canonical dialog from the Settings window route but never consumes
the flow/profile query. The daemon persists that window route; a reload rehydrates and raises it again.
The owning invariant is one-time consumption of a palette action across cancellation and reload.
The existing E2E-027 in web/e2e/__tests__/profiles.spec.ts owns the real window-route/dialog composition.

## Repair and verification

The Settings window owns both raising the dialog and consuming its route intent. Its existing
window-manager replacement removes only flow/profile after a valid local lifecycle intent is
raised. This persists the consumed route without adding a browser history entry. Unknown flows
and incomplete target intents remain ignored. The route bridge moves into the owning Settings
app; the Profiles page no longer receives navigation callbacks or lifecycle search props.

E2E-027 fails before repair because Cancel retains the action URL for the full 20-second assertion
budget. On the final production Web build, E2E-017 and E2E-027 pass against a real daemon, including
history dismissal, cancellation, reload and independently active ownership. Receipts:
profile-palette-cancel-red.log and profile-lifecycle-final-{web-build,e2e}.log. React Doctor returns
100/100 with no issues in profile-lifecycle-final-doctor.log; no diagnostic suppression is added.

Fresh Sol replay starts from the previously retained deep link. It opens once, consumes the query,
and stays canceled after reload. A new palette Archive invocation also opens the canonical dialog
with a clean URL; Escape cancellation survives another reload. Independent UDS retains the active
navigation-notes identity. The 22-frame profile-palette-cancel-fixed-sol recording is stopped,
and profile-palette-cancellation-persists.png is inspected. Source is not read during the walk.
Receipts: profile-palette-cancel-fixed-sol-*. The full lifecycle charter and spoken VoiceOver
remain Pending; these bounded replays do not close the remaining scope.
