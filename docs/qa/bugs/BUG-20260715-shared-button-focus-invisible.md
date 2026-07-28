# BUG-20260715-shared-button-focus-invisible: Keyboard focus is imperceptible on dark controls

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-enable-coordinated-conversations, paginate the run conversation by keyboard
- **Scenarios:** NB-run-conversation-bounds-usage
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

The shared outward focus token painted only a one-pixel `--color-line-strong` hairline at 9% white alpha. `Load older messages` received `:focus-visible`, but its focus indicator was not perceptible against the dark canvas. Every shared primitive using `shadow-focus-ring` inherited the same defect.

## Reproduction

- **Charter:** CH-coordination-future-runs · **Tour:** Back-Button Tour
- **Environment:** 375/768/1280 desktop-class viewports / isolated local daemon / en-US

1. Open a run conversation with paginated history.
2. Press Tab until `Load older messages` receives focus.
3. Inspect the rendered control at each required width.

**Expected:** Keyboard focus has a persistent, non-color-only, contrast-safe boundary distinct from the unfocused control.
**Actual:** `:focus-visible` matched, but the one-pixel low-alpha hairline was visually indistinguishable from the resting border.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-coordination-future-runs.md`
- `qa/screenshots/coordination-run-focus-fixed-{375,768,1280}.png` in the isolated lab.

## Fix

- **Root cause:** `--shadow-focus-ring` was designed as an active-control hairline rather than a keyboard focus indicator.
- **Fix commit:** pending final whole-diff commit.
- **Regression owner:** the canonical design token/codegen contract plus real keyboard visual evidence. No CSS-literal unit test was added because it would freeze an implementation detail rather than prove rendered focus visibility.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** the shared token now paints a 1 px canvas separator and a 2 px `#f6874f` outer edge. After the 140 ms transition, computed style and screenshots at 375/768/1280 show the ring; contrast against the dark surface ramp is at least 6.43:1.
