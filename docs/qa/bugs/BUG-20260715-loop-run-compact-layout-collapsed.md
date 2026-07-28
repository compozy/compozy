# BUG-20260715-loop-run-compact-layout-collapsed: Loop run form collapses below desktop

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Nia
- **Journey Step:** J-network-local-default, choose Loop participation on a compact device
- **Scenarios:** NB-execution-participation-defaults; NB-participation-controls-serialize
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Below the desktop breakpoint, the Loop run grid allocated only 40 px to a 590 px form while the preview consumed the remaining track. The participation fields existed but became unreachable or vanished when scrolled into view.

## Reproduction

- **Charter:** CH-network-local-default · **Tour:** Feature Tour
- **Environment:** 375 px / isolated daemon-served Web / en-US

1. Open a Loop run page at 375 px.
2. Select Live and try to inspect channel, strategy, and preview.

**Expected:** The form flows before the preview under one compact scroll owner.
**Actual:** the form measured `clientHeight=40`, `scrollHeight=590`; scrolling to Participation displaced its controls and left mostly the preview visible.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md`
- Lab screenshots: `qa/screenshots/loop-run-participation-{local,live}-{375,768,1280}.png`.

## Fix

- **Root cause:** the one-column compact grid retained two independently scrolling children, so implicit row sizing starved the form.
- **Fix commit:** pending final whole-diff commit.
- **Regression owner:** real responsive layout and the existing `LoopRunForm` Storybook story; the existing form suite owns participation serialization. No CSS-literal test was added.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** at 375 px the form measures 741/741 px inside a 764/1490 px scroll container, document width remains 375/375, and Local/Live controls plus the preview are reachable in order. Tablet and desktop layouts also pass.
