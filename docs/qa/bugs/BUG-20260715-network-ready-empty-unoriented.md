# BUG-20260715-network-ready-empty-unoriented: Ready Network empty state gives no Local/Live orientation

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Nia
- **Journey Step:** J-network-local-default, discover the Network without enabling it
- **Scenarios:** NB-network-empties-onboarding-settings
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

The ready Network route rendered the generic `No channels yet` empty state. It did not explain that ordinary executions remain Local, that Live is selected per future run, or that Network availability never enrolls an execution.

## Reproduction

- **Charter:** CH-network-local-default · **Tour:** Feature Tour
- **Environment:** 375/768/1280 px / isolated daemon-served Web / en-US

1. Enable Network in a fresh workspace with no channels.
2. Open `/network`.

**Expected:** The empty state orients the operator to Local-by-default semantics and offers one neutral Settings action.
**Actual:** A generic channel-empty message offered no participation guidance.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md`
- Lab screenshots: `qa/screenshots/network-ready-empty-before-1280.png` and `qa/screenshots/network-ready-empty-after-{375,768,1280}.png`.

## Fix

- **Root cause:** the real Network route bypassed the existing state-aware `NetworkEmpty` composite and mounted the generic `Empty` primitive.
- **Fix commit:** pending final whole-diff commit.
- **Regression tests:** the canonical Network empty-state component suite and daemon-served Network E2E route own ready/disabled semantics and CTA navigation.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** ready and disabled states render truthful Local/Live copy, one `Open Network settings` action, visible keyboard focus, and no horizontal overflow at all three widths.
