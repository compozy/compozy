---
id: TA-loop-failure-breaker
area: TA
title: Bound an unbounded failing Loop watch
persona: Ada
journey: J-bound-runaway-work
expected: An unbounded watch with consecutive failed generations stops at the circuit-breaker backstop, while a healthy watch remains active and ordinary repeated node failures quarantine instead of ending the run.
entry_points: `compozy loop status --run-id <run-id> -o json`; `compozy__loop_status`; Loop run events over HTTP/SSE
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-ta-replay-20260730-062156-531636-lab/qa-artifacts/qa; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/adjacent-safety-tests.json
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
overlaps: LP-sick-target-degrades-one-lane; LP-quarantine-diagnose-requeue; LP-029; LP-031; LP-038; TA-action-run-liveness
---

Added by the Hermes comparison breaker milestone. Exercise a sibling success after the failing node,
the inverse terminal order, an unbounded watch with consecutive failed generations, and a healthy
watch control.

Flag only: the later Hermes comparison QA cycle owns execution and evidence.

Phase C planning 2026-07-19: linked to J-bound-runaway-work; settles US-013 (O2, Safety
Invariant 23).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Generation log showing the per-node streak trip to `Stalled` (not the iteration cap), in both
  terminal orders.
- The hard-backstop termination record for the unbounded failing watch, and the healthy-loop
  control run that never trips.

QA impact 2026-08-01: reset to `untested` because node failure and gate rejection now use distinct
reattempt/succession semantics. Re-walk both node strategies plus revise and fresh-next-generation
controls before relying on the earlier breaker verdict; historical evidence remains cited above.

QA impact 2026-08-02: Task 04 replaces per-node terminal breaker behavior with quarantine while
retaining the unbounded-watch backstop. A public walk is blocked until Task 07 exposes quarantine,
breaker-transition events, and node requeue through structured CLI/HTTP/UDS surfaces. Historical
breaker evidence remains cited above but does not prove the revised contract.
