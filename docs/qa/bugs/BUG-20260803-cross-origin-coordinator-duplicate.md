# BUG-20260803-cross-origin-coordinator-duplicate: Internal coordinator origins collided on one deterministic run

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-recover-loop-node-failure, resume after daemon restart
- **Scenarios:** LP-durable-wait-restart; LP-live-pause-repair-resume
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle.md

## Summary

Boot recovery and a node-terminal hook could reserve the same deterministic next coordinator run
under different internal origins. Origin-scoped idempotency missed the existing row, then the run
ID uniqueness constraint rejected the second reservation and left progress behind an active lease.

## Reproduction

- **Charter:** CH-operator-loop-recovery · **Tour:** Interrupt Tour
- **Environment:** isolated macOS lab / desktop / wifi-fast / en-US

1. Restart with a Loop generation that needs another coordinator pass.
2. Let `daemon.boot` reserve the deterministic generation coordinator.
3. Deliver a node-terminal wake from `loop.hooks` for the same generation and run ID.

**Expected:** Both internal causes coalesce onto the same semantic coordinator reservation.

**Actual before the fix:** The second reservation failed the task-run ID uniqueness constraint;
the generation could remain live without a claimable coordinator.

## Evidence

- Lab run `looprun-2e361dc5f104a10f`, generation 3.
- Regression: `Should coalesce one deterministic coordinator run across internal origins` in the
  canonical global DB coordinator suite.

## Fix

- **Root cause:** Coordinator reservations checked only the idempotency tuple, whose origin is
  intentionally different for boot and hook paths, before inserting a globally deterministic run
  ID.
- **Fix:** Look up the deterministic run ID first. Coalesce only when task, run kind, Loop run ID,
  and idempotency key all match; reject any conflicting identity as validation failure.
- **Fix commit:** pending Task 13 checkpoint
- **Regression test:** `internal/store/globaldb/global_db_task_claim_test.go` reproduces the boot →
  hook origin sequence and proves the existing run is reused without a duplicate row.

## Verification

- **Retested:** 2026-08-03 by rebuilding and restarting the same isolated lab.
- **Result:** Pass. Boot settled with one started coordinator and no active or queued duplicate
  coordinator rows.
