# BUG-20260729-accepted-start-stop-identity-race: Immediate stop split session creation identity

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-17, stop a newly accepted session before provider activation
- **Scenarios:** RT-new-session-fast-feedback
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated HTTP create/stop replay while preparing RT-011 pagination

## Summary

Stopping a session immediately after its durable `201 starting` response could cancel startup before
the event recorder was attached or while the immutable creation identity was being projected. The
stop either returned 500 despite reaching `stopped/user_canceled`, or left complete identity metadata
in `meta.json` while the global catalog retained null identity columns. The next daemon boot then
failed closed with `session_creation_identity_mismatch`.

## Reproduction

1. Create a session through `POST /api/sessions` and retain its accepted session ID.
2. Immediately call `POST /api/workspaces/:workspace_id/sessions/:session_id/stop`.
3. Repeat, then restart the daemon.

**Expected:** Every stop returns 204, produces one terminal event, and leaves meta/catalog in the same
identity state; daemon reconciliation succeeds.

**Actual before the fix:** One stop returned 500 with `event recorder is not available`. Seven of eight
fixtures persisted complete identity only in `meta.json`; boot reconciliation rejected the split state.

## Evidence

- Pre-fix diagnostics, reversible fixture quarantine, repaired replay, and clean second boot:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/035-accepted-start-stop-race`.

## Fix

- **Root cause:** durable acceptance returned before recorder attachment, while the later launch-identity
  write and the terminal transition had no shared ownership boundary.
- **Correction:** starting-session stop waits for recorder readiness and shares a cancelable launch-commit
  permit with effective-permission, identity, meta, and catalog persistence. The winner commits a complete
  identity or aborts it consistently before terminal state; partial cross-authority identity is impossible.
- **Fix commit:** pending completion gate
- **Regression test:** the existing
  `TestStopJoinsAcceptedStartupAndPreventsLateActivation` suite now owns recorder readiness and an
  identity-commit overlap using the real global catalog.

## Verification

- The regression failed red when Stop projected `stopping` during a blocked identity registration.
- The repaired suite and acceptance-latency suite passed 10 consecutive race-enabled repetitions.
- `make lint` and `make build` pass.
- Twenty immediate HTTP create/stop pairs returned `201 → 204`; the canary also returned 204.
- All 21 fixtures are `stopped/user_canceled`: 11 have byte-matching complete identities, 10 have
  identities absent from both authorities, and zero are mismatched.
- A second daemon boot completed reconciliation with zero indexed and zero orphaned sessions.
- **Retested:** rebuilt candidate green; governed fix commit pending
