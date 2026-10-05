# BUG-20261004-scheduler-controls-stale: Queue controls keep showing Running after dispatch pauses

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-drain-scheduler, monitor admitted work during maintenance
- **Scenarios:** TA-048
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora keeps Tasks Dashboard open while draining through another public surface. The active-run
cards change, but Queue controls still says Running and shows zero waiting runs. Reload reveals
the persisted Paused state and the queued follow-up. The controls contradict the maintenance state.

## Reproduction

- **Charter:** CH-scheduler-drain-recovery · **Tour:** Interrupt Tour
- **Environment:** desktop, 1512 × 862, local Wi-Fi, en-US; isolated real daemon

1. Open Tasks Dashboard and let its initial scheduler reads settle.
2. Start a real task and drain through HTTP while it is working.
3. Queue a follow-up while dispatch is paused; inspect Queue controls without reloading.
4. Compare with fresh UDS scheduler reads, then reload the same dashboard.

**Expected:** The open controls reconcile external pause, active claims and backlog changes.
**Actual:** The initial Running/zero/empty projection persists until reload.

## Evidence

Receipts in docs/qa/evidence/2026-10-02-untested/:
- scheduler-drain-dora-replay-web-midwait.png and web-midwait-state.json
- scheduler-drain-dora-replay-web-scheduler-requests.json: only the initial scheduler reads
- scheduler-drain-dora-replay-mid-wait.json: paused=true, one active claim, one queued run
- scheduler-drain-dora-replay-web-reload.json: the same controls now show Paused
- scheduler-drain-dora-replay-ended.json

## Fix

- **Root cause:** scheduler status/backlog had a freshness window but no refresh cadence or
  external event subscription. Only local mutations invalidated them. They now reuse the
  Dashboard's 30-second cadence and its existing inactive-window admission flag.
- **Fix commit:** 8d630a7f0
- **Regression test:** use-tasks-page.test.tsx observes changed adapter responses without
  remount/reload and confirms no reads after deactivation. It fails before and passes after repair.

## Verification

Dora repeats the original external drain and follow-up enqueue while Dashboard stays open.
Queue controls changes to Paused with one waiting run and the correct backlog row without a
reload. After she withdraws the follow-up through CLI, Web Resume restores dispatch; a fresh
UDS read reports paused=false with zero active claims and zero queued runs. A final reload
shows Running. scheduler-projection-fixed-dora-live-controls.png and restored.png were inspected;
the 14-frame recording is closed in scheduler-projection-fixed-dora-ended.json. Delivery gate
and commit are pending. The remaining initial profile-preload defect is tracked separately.

## Delivery closure — 2026-10-04

Commit 8d630a7f0 records the repaired behavior. The original-persona replays above and all selected
local gate lanes pass. scheduler-profile-final-delivery-gate.json, final-delivery-gate-status.json
and commit-proof.json retain current-input verification; the committed tree exactly matches
the checked tree 93e4b5183de281b741b373fc93ab802bc89c27f5. Broader QA and PR/CI remain separate.
