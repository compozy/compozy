# BUG-20260826-session-cli-profile-scope: Session prompt and stop ignored the selected profile

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, operate a profile-owned session from the CLI
- **Scenarios:** ET-profile-selection-precedence; ET-profile-web-aggregate-owner-surfaces
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-profile-identity-final.md

## Summary

Ada could create and list a session under a named profile, but `session prompt` and `session stop` searched the default profile and returned `session not found`.

## Reproduction

- **Charter:** CH-profiles-final · **Tour:** Profile-owned session mutation
- **Environment:** macOS terminal, isolated local daemon, en-US

1. Create a session under `archive`.
2. Run `compozy --profile archive session prompt` for that session.
3. Run `compozy --profile archive session stop` for that session.

**Expected:** Both mutations reach the session in `archive`.
**Actual:** Both commands returned `session not found` because the client used the default profile.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/usage-all-profiles-breakdown.png`
- The live retest returned a completed Codex response from `session prompt`, then returned the stopped session record from `session stop` under `archive`.

## Fix

- **Root cause:** The prompt and stop Cobra commands did not install the profile-aware client mutation hook used by other session mutations.
- **Fix commit:** this remediation batch
- **Regression test:** `internal/cli/session_test.go` — the canonical session CLI suite asserts selected-profile propagation for prompt and stop.

## Verification

- **Retested:** 2026-08-26, same persona/journey · **Report:** docs/qa/reports/2026-08-26-profile-identity-final.md
- **Result:** Both commands resolved the `archive` session, the provider prompt completed, and stop returned the same profile-owned session.
