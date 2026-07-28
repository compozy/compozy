# BUG-20260712-bridge-e2e-retired-route: Bridge browser scenario still targets the retired two-pane route

- **Status:** fixed
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P2
- **Persona Affected:** Bruno and Ada as release operators maintaining bridge browser coverage
- **Journey Step:** J-complete-web-bridge-setup complete bridge setup in the Web
- **Scenarios:** NB-026; NB-028; NB-039; NB-web-bridge-setup (automation evidence only; user verdicts unchanged)
- **Found:** 2026-07-12 · **Report:** docs/qa/reports/2026-07-12-hermes-bridge.md
- **Origin:** n/a

## Summary

After the current bundle was served, the Bridge Playwright owner still expected the retired split-pane screen. It searched for standalone scope pills, required list and detail panels simultaneously, and looked for a newly created list row after the application had already navigated to `/bridges/:id`.

## Reproduction

- **Charter:** CH-web-bridge-setup automated owner · **Tour:** Back-Button Tour precondition
- **Environment:** local Linux, rebuilt `web/dist`, isolated daemon and Telegram reference adapter

1. Run `bridges.spec.ts` against the current daemon-served bundle.
2. Create a bridge through the wizard.
3. Observe the route and the current catalog/detail layout.

**Expected:** The scenario follows `/bridges` catalog → `/bridges/:id` detail, verifies disabled-first setup, and asserts the human-readable status copy while retaining exact API enum checks.
**Actual:** The scenario waits for removed `bridge-scope-all`, then searches for list rows and the list panel while already on the detail route. It also expects raw `auth_required` text even though the UI intentionally renders `auth required`.

## Evidence

- Current UI capture shows the catalog with Filter controls and the bridge detail on its own route.
- The create response and public API report `enabled: false` / `disabled`; the obsolete assertion failed only because it searched the wrong surface.
- The ingress helper observed `route_count >= 1` and a persisted route before the stale list assertion.

## Fix

- **Root cause:** the Task 05 catalog/detail route redesign and humanized status copy did not co-ship updates to this older release E2E owner because BUG-0037 kept the lane on an old bundle.
- **Fix commit:** Task 10 checkpoint; SHA will be backfilled because a commit cannot contain its own hash.
- **Regression test:** the existing `bridges.spec.ts` now follows public navigation, asserts the route on the detail surface, keeps exact HTTP/UDS/CLI lifecycle assertions, and checks detail responsiveness at mobile, tablet, and desktop widths.

## Verification

- **Retested:** 2026-07-12, focused daemon-served Playwright.
- **Result:** the create/edit/enable/ingress route scenario passed in 13.7 seconds; the create/secret rotation/auth failure/restart recovery scenario passed in 15.4 seconds.
