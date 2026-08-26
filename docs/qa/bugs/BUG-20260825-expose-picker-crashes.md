# BUG-20260825-expose-picker-crashes: The Expose panel crashes when targets are available

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-share-skills-with-other-tools, choose a provider target
- **Scenarios:** ET-web-skill-expose-panel
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

Bruno opens a skill that can be exposed to another tool, but Marketplace replaces the whole window
with “Marketplace failed to render.” The failure appears only when at least one target exists, so
empty-state tests do not reveal it.

## Reproduction

- **Charter:** CH-skill-expose-web-repair · **Tour:** Back-Button Tour
- **Environment:** desktop / 1280×900 / wifi-fast / en-US, daemon-served production bundle

1. Enable the `agents` skill source.
2. Open a Compozy-native installed skill.
3. Expand Exposures.

**Expected:** “Expose to…” opens a target picker.
**Actual:** the Marketplace error boundary reports Base UI error 31 and the target picker never
renders.

## Evidence

- /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e
- Base UI error 31 resolves to a menu group label rendered without its required menu group.

## Fix

- **Root cause:** `DropdownMenuLabel` wraps Base UI's `Menu.GroupLabel`, but the expose picker
  rendered the label and checkbox items directly inside the popup without `DropdownMenuGroup`.
- **Fix commit:** `df739b0`
- **Regression test:** `web/e2e/__tests__/skill-sources.spec.ts`, E2E-011. The production bundle
  failed before the fix and passes after it.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** the picker renders, creates the link, detects its external deletion, repairs it, and
  refuses to touch a foreign entry.
