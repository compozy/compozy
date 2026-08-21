# BUG-20260821-sessionless-lease-recovery: Boot cannot recover a daemon-owned lease without a session

- **Status:** verified
- **Impact (user-side):** Blocks-Startup
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada, runtime operator
- **Journey Step:** J-loop-terminal-recovery, boot after a daemon crash
- **Scenarios:** LP-terminal-loop-settlement
- **Found:** 2026-08-21 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md

## Summary

Boot failed closed while recovering expired coordinator run `run-1dd863718d2fbf80`. Its
`session_id` was correctly absent, but the optimistic SQL guard compared `NULL = NULL`, which is
never true in SQLite, and then reported the existing run as missing.

## Evidence

- Failure: `qa/settlement/boot-after-sessionless-fix.log`
- Seed: `qa/settlement/sessionless-coordinator-seed.json`
- Retest: `qa/settlement/boot-final-rewalk-2.log`

## Fix

- **Root cause:** The lease transition guard was not null-safe for `session_id`.
- **Fix commit:** `0a4fe2d`
- **Regression test:** `TestGlobalDBRecoverExpiredRunLeasesThenClaim`

## Verification

- **Retested:** 2026-08-21 in the isolated runtime lab
- **Result:** The exact sessionless lease passed the guard and entered boot recovery.
