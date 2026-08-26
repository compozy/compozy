# BUG-20260826-profile-palette-worktree-owner: Worktree search omitted its profile owner

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-scope-work-by-profile, command-palette worktree search
- **Scenarios:** ET-profile-web-aggregate-owner-surfaces
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-profile-identity-final.md

## Summary

Ada could find a worktree from another profile in the command palette, but the result did not identify its owner even though worktrees intentionally remain visible across profile scopes.

## Reproduction

- **Charter:** CH-profiles-final · **Tour:** Worktree ownership
- **Environment:** macOS desktop, 1440×900 viewport, local network, en-US

1. Adopt a worktree under `default`.
2. Switch to `research` and open the command palette.
3. Search for the adopted worktree.

**Expected:** The worktree row names `default` in every profile lens.
**Actual:** The worktree appeared without an owner label.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/worktree-owner-default.png`
- The same worktree remained discoverable from `research` after a fresh palette load.

## Fix

- **Root cause:** The worktree domain result types and both palette render paths omitted `profile_name`.
- **Fix commit:** this remediation batch
- **Regression test:** `web/src/systems/os/hooks/__tests__/use-os-palette-root.test.tsx` — the canonical root-palette suite now asserts worktree ownership in the profile-scoped lens.

## Verification

- **Retested:** 2026-08-26, same persona/journey · **Report:** docs/qa/reports/2026-08-26-profile-identity-final.md
- **Result:** The worktree row displayed `default` from the `research` lens and remained visible as required.
