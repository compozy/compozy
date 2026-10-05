# BUG-20261005-loop-mobile-run-names-truncated: Mobile Runs rows hide their identifying names

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Marina
- **Journey Step:** J-03, find and decide a waiting human gate
- **Scenarios:** LP-008
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

At phone width the first table cell shrinks until Loop names and summary context show only a few letters. Multiple studio runs become indistinguishable.

## Reproduction

CH-002, Interrupt Tour, 430x932 touch viewport at DPR 3, 4G, en-US.
Use Marina's 430x932 touch viewport with 4G emulation. Open /loop-runs with a waiting review run and several recent Studio runs; inspect Needs you and Recent.

## Evidence

loops-marina-approval-runs.png and loops-marina-approval-initial-triage.json. The Needs you row shows st... and an ...; repeated Recent names truncate identically.
All receipts are under docs/qa/evidence/2026-10-02-untested/. The exact twelve-frame
loops-review-marina recording is closed. No provider workers or mocked services are involved.

## Fix

The name cell uses w-full max-w-0 without a readable minimum, while sibling columns consume the narrow viewport.

Fix commit: pending delivery gate.
Reserve the existing table's name cell at 256px minimum width and retain its shared
horizontal overflow. Marina's touch replay reads both studio-onboarding-review and
studio-release-repair-readiness in full; a real gesture scrolls the remaining columns
225px without widening the page. At 1512px the table fits without horizontal overflow.
Evidence: loops-human-review-runs-mobile.json, loops-human-review-mobile-columns.png,
loops-human-review-runs-desktop.json, and loops-runs-restored-marina-verified.png.
