# BUG-20260822-requeued-continuation-epoch-drift: Requeued continuation keeps stale output epoch

- **Status:** verified
- **Impact (user-side):** Blocks-Operation
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada, Loop operator
- **Journey Step:** Repair a quarantined node, then start another Loop
- **Scenarios:** Node requeue lifecycle; ordered run summaries across HTTP, UDS, and CLI
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

Requeue fenced the retained continuation output by incrementing its epoch, but copied the older epoch
from task metadata into the replacement task run. Its valid completion was then classified as a late
arrival. The resulting coordinator retained the daemon-wide coordinator lease and prevented a later
Loop kickoff from progressing.

## Reproduction

- **Invariant:** A retained continuation released by requeue carries the same epoch as its persisted
  generation output, so valid completion settles and later Loop coordinators remain claimable.
- **Owning layer:** Global Loop runtime persistence.
- **Canonical suite:** `TestGlobalDBLoopNodeRequeueShouldBeAtomic`.
- **Release lane:** `TestDaemonE2ELoopRunReadCLIJourneys`, where IT-027 precedes IT-032.
- **Red evidence:** `~/Library/Application Support/rtk/tee/1787387399_go_test.log` and
  `qa/test-e2e-runtime-final.log` under the task_07 QA output.
- **Forensics:** `qa/diagnostics/loop-read-stuck-coordinator.sample.txt` under the task_07 QA output.

## Fix

The requeue reservation now reads the fenced epoch from `loop_generation_outputs` and writes that
epoch into the replacement run metadata while preserving all other task metadata.

## Verification

- Canonical requeue suite: 7 passed with the replacement worker reaching `succeeded`.
- Cumulative daemon run-read journey and the full runtime E2E lane are rerun as release evidence.
