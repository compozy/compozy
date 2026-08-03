# BUG-20260803-loop-boot-active-coordinator-lease: Daemon boot failed while a Loop coordinator lease was active

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-recover-loop-node-failure, restart recovery
- **Scenarios:** LP-durable-wait-restart; LP-crash-death-resume
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle.md

## Summary

Restarting the daemon while a Loop coordinator still held its consumer lease could abort startup.
The scheduler backstop treated the expected active-consumer signal as a fatal error instead of
leaving the queued coordinator for the current consumer.

## Reproduction

- **Charter:** CH-operator-loop-recovery · **Tour:** Interrupt Tour
- **Environment:** isolated macOS lab / desktop / wifi-fast / en-US

1. Start Loop runs that leave coordinator work queued and one coordinator claimed.
2. Terminate the daemon abruptly.
3. Restart it against the same isolated `COMPOZY_HOME`.

**Expected:** Boot succeeds and the active consumer owns the lease until it is released or
recovered.

**Actual before the fix:** The scheduler backstop returned `ErrActiveRunLease`; daemon startup
failed before the restored runs could progress.

## Evidence

- Fresh-lab restart against the preserved timer, retry, and approval runs.
- Regression: `TestSchedulerTaskSourceLoopCoordinatorBackstopShouldDeferWhileConsumerLeaseIsActive`.

## Fix

- **Root cause:** `RunLoopCoordinatorBackstop` handled workspace capacity and an empty queue as
  normal backpressure, but did not handle the equivalent global coordinator-consumer lease.
- **Fix:** Treat `ErrActiveRunLease` as scheduler backpressure and leave the queued run untouched.
- **Fix commit:** pending Task 13 checkpoint
- **Regression test:** `internal/daemon/scheduler_runtime_test.go` proves the claimed coordinator
  remains active and the second coordinator remains queued without failing the backstop.

## Verification

- **Retested:** 2026-08-03 in the same isolated lab after rebuilding the daemon.
- **Result:** Pass. Abrupt restart completed, all six coordinator families initialized, and the
  durable timer wait kept its exact `resume_at` value.
