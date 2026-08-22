# BUG-20260822-retained-continuation-coordinator-selection: Requeue selects a fresh-generation planner for retained work

- **Status:** verified
- **Impact (user-side):** Blocks-Operation
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada, Loop operator
- **Journey Step:** Repair a quarantined node, then start another Loop
- **Scenarios:** Node requeue lifecycle; ordered run summaries across HTTP, UDS, and CLI
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

When a quarantined next-generation continuation existed without an open coordinator, requeue chose
the fresh-generation planner. That planner expected epoch zero while the retained continuation had
already been fenced. Coordinator completion failed, retained its daemon-wide lease, and left later
Loop kickoffs queued at generation zero.

## Reproduction

- **Invariant:** Requeue selects the retained-continuation path from persisted continuation state,
  regardless of whether a coordinator was already open.
- **Owning layer:** Global Loop requeue transaction.
- **Canonical suite:** `TestGlobalDBLoopNodeRequeueShouldBeAtomic`.
- **Release lane:** `TestDaemonE2ELoopRunReadCLIJourneys`, where IT-027 precedes IT-032.
- **Red evidence:** `~/Library/Application Support/rtk/tee/1787388830_go_test.log` and
  `qa/test-e2e-runtime-after-epoch-fix-failure.log` under the task_07 QA output.
- **Forensics:** `qa/diagnostics/loop-read-generation-zero-block-daemon.log` under the task_07 QA
  output; the log records `stale generation output: output primary/0 expected epoch 0`.

## Fix

The transaction now detects retained continuation rows directly. It preserves or creates their
generation provenance, reuses an open generation coordinator or reserves a new wake, advances the
cursor, and reserves replacement worker runs with the fenced epoch.

## Verification

- Canonical requeue suite: 7 passed, including retained work with no pre-opened coordinator.
- Cumulative daemon run-read journey and the full runtime E2E lane are rerun as release evidence.
