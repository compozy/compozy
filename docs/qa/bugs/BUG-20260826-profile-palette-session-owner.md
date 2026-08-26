# BUG-20260826-profile-palette-session-owner: Aggregate session search hid its owning profile

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-scope-work-by-profile, aggregate command-palette search
- **Scenarios:** ET-profile-web-aggregate-owner-surfaces
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-profile-identity-final.md

## Summary

Ada could find sessions from several profiles in the aggregate command palette, but the rows did not name their owners. Sessions with similar titles were therefore indistinguishable.

## Reproduction

- **Charter:** CH-profiles-final · **Tour:** Aggregate ownership
- **Environment:** macOS desktop, 1440×900 viewport, local network, en-US

1. Create sessions under `default` and `research`.
2. Select All profiles and open the command palette.
3. Search for both session titles.

**Expected:** Every aggregate session result names its owning profile.
**Actual:** Both results appeared without an owner label.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/aggregate-command-palette-session-owner-fixed.png`
- A fresh aggregate palette load showed `default` and `research` beside their session rows.

## Fix

- **Root cause:** The aggregate entity mapper discarded `profile_name`, so the row renderer had no owner to display or search.
- **Fix commit:** this remediation batch
- **Regression test:** `web/src/systems/os/hooks/__tests__/use-os-palette-root.test.tsx` — the canonical root-palette suite now asserts the owner on aggregate session results.

## Verification

- **Retested:** 2026-08-26, same persona/journey · **Report:** docs/qa/reports/2026-08-26-profile-identity-final.md
- **Result:** Fresh aggregate search labeled both session rows with their owning profiles; the scoped palette remained free of redundant owner labels.
