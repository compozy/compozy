# BUG-20260928-derive-replay-deleted-child-origin-lost: Retrying a continue after deleting the child loses the origin agent

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Théo
- **Journey Step:** J-15-operate-session-via-cli-api, lost-response retry after the child was removed
- **Scenarios:** RT-session-derive-retry
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b1.md

## Summary

After `compozy session remove <child>`, repeating the same `session continue … --idempotency-key` returned
`Continued <src> (--) into <child> (deleted)` and `Origin continue · from <src> · --`: `origin_agent_name` was empty although the
recorded outcome had it.

## Root cause

The derive receipt outcome (`store.SessionDerivationOutcome`) did not record the origin agent; the replay read it from the child's
lineage, which no longer exists once the child is deleted.

## Fix

`origin_agent_name` is recorded on the receipt outcome (`internal/store/types_session_derive.go`, `deriveOutcome` in
`internal/session/derive_prepare.go`) and mapped back in `deriveResultFromReceipt` (`internal/session/derive_commit.go`).
Receipts written before the fix (none released) keep the old empty value. Regression: `TestContinueSession` retry case
(`internal/session/manager_derive_test.go`) asserts the recorded origin agent after deletion.

## Verification

Re-walk with a fresh key: `Continued <src> (retry-source) into <child> (deleted)`, `Child deleted yes` (`docs/qa/evidence/2026-09-28-session-continue-fork-b1/retry-5b.txt`).
