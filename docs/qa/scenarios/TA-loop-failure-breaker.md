---
id: TA-loop-failure-breaker
area: TA
title: Stall persistent Loop failures without sibling resets
persona: Ada
journey: J-bound-runaway-work
expected: A two-node Loop with one repeatedly failing node and one healthy sibling stalls with circuit_breaker at the per-node limit regardless of terminal order; an unbounded failing watch also stalls, while a healthy watch remains watching.
entry_points: `agh loop runs show <run-id> -o json`; `agh__loop_status`; Loop run events over HTTP/SSE
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-029; LP-031; LP-038; TA-action-run-liveness
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
