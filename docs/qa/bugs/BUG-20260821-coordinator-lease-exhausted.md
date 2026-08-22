# BUG-20260821-coordinator-lease-exhausted: Expired Loop coordinator is exhausted as ordinary work

- **Status:** verified
- **Impact (user-side):** Blocks-Startup
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada, runtime operator
- **Journey Step:** J-loop-terminal-recovery, resume a running Loop after a crash
- **Scenarios:** LP-terminal-loop-settlement
- **Found:** 2026-08-21 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md

## Summary

After lease recovery became null-safe, the expired coordinator reached the ordinary task retry
limit and became `needs_attention`. Loop reconciliation then tried to enqueue a duplicate
coordinator and failed closed. Coordinator retries belong to the Loop lifecycle, not task attempt
exhaustion.

## Evidence

- Failure: `qa/settlement/boot-after-sessionless-fix.log`
- Successful recovery and terminal sweep: `qa/settlement/boot-terminal-sweep.log`
- Idempotent replay: `qa/settlement/boot-idempotent-second.log`

## Fix

- **Root cause:** Expired coordinator leases used the ordinary task exhaustion branch.
- **Fix commit:** `69c2d74`
- **Regression test:** `TestGlobalDBRecoverExpiredRunLeasesThenClaim`

## Verification

- **Retested:** 2026-08-21 in the isolated runtime lab
- **Result:** The coordinator is requeued, the terminal sweep settles three records and repairs one
  orphan, and a second boot settles zero records with no duplicate repair event.
