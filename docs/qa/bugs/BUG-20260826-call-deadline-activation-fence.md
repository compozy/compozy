# BUG-20260826-call-deadline-activation-fence: Deadline settlement leaked activation fence internals from call creation

- **Status:** fixed, pending public-surface retest
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-delegate-work-to-an-agent — create a call with an opt-in deadline
- **Scenarios:** RT-agent-call-deadline-timeout
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-agent-comms.md`

## Summary

A short-deadline call could durably settle as `timeout` while its public create command failed with
internal activation-fence and claim-token errors. The call existed and had the correct terminal
record, but the caller received an infrastructure failure instead of that record.

## Reproduction

1. Create a call whose child takes longer than an opt-in one-second deadline.
2. Let the deadline sweeper win after child creation but before activation binding commits.
3. Inspect the CLI result and then read the durable call directly.

**Expected:** Create returns the durable terminal `timeout` call, and the unbound child is stopped.
**Actual:** Create returned `settlement was fenced` joined with `invalid claim token`, while the
durable call `call-a2dd5c68719b0a90` was already `timeout` with `call_timeout`.

## Fix

- **Root cause:** Activation binding treated a deadline settlement that had already fenced and
  cleared the activation claim as an unexpected persistence failure. It then tried to release the
  claim a second time, which joined a misleading claim-token error into the public response.
- **Production fix:** Activation-run fencing now reports the typed `call_already_settled` outcome.
  The service stops any just-created unbound child, reloads the durable call, returns its terminal
  state, and does not release a claim already owned by the settlement winner.
- **Regression:** The canonical call settlement suite deterministically terminalizes the call
  immediately before bind, then proves Create returns `timeout`, the orphan child is stopped once,
  and no second lease release occurs.
- **Fix commit:** pending QA remediation commit.

## Verification

- Focused red/green scope: `go test -race ./internal/calls ./internal/store/globaldb -run 'TestServiceCancelAwaitDeadlineAndDrain|TestGlobalDBCallActivationClaimCancelRaceHasOneFencedOutcome' -count=1` — 13 tests passed.
- Public CLI retest in the isolated lab is pending a rebuilt daemon.
- Reproduction evidence: `/Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/call-deadline-activation-fence-reproduction.md`.
