# BUG-20260715-automation-editor-compact-layout-clipped: Automation editor clips compact layouts

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Nia
- **Journey Step:** J-network-local-default, inspect automation controls on compact devices
- **Scenarios:** NB-participation-controls-serialize
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

The Automation editor exceeded the viewport at tablet width, collapsed its editor track to zero height in compact mode, and allowed segmented controls to widen the mobile form. Important target, participation, owner, and schedule controls were clipped or unreachable.

## Reproduction

- **Charter:** CH-network-local-default · **Tour:** Feature Tour
- **Environment:** 375/768 px / isolated daemon-served Web / en-US

1. Open Create job or Create trigger below the desktop breakpoint.
2. Select task-backed Job fields or scroll the Trigger editor.

**Expected:** One scroll owner contains the editor and preview within the viewport.
**Actual:** the 768 px dialog rendered from `left=-206` to `right=974`; compact grid rows allocated zero height to the editor; mobile content widened beyond its 343 px dialog.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md`
- Lab screenshots: `qa/screenshots/automation-task-participation-*.png` and `qa/screenshots/automation-trigger-compact-375.png`.

## Fix

- **Root cause:** a fixed modal max width, competing compact grid rows/child scroll containers, implicit min-content sizing, and non-wrapping target/schedule groups combined at the responsive boundary.
- **Fix commit:** pending final whole-diff commit.
- **Regression owner:** rendered responsive contract via daemon-served browser and Storybook captures; existing Job/Trigger form and editor suites retain interaction/serialization ownership. No CSS-literal test was added.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** the 375 px dialog is exactly 343/343 px, the 768 px dialog stays inside 16–752 px, 1280 px preserves the split preview, and Job/Trigger editor scroll remains reachable with zero document overflow.
