---
id: LP-sick-target-degrades-one-lane
area: LP
title: Keep healthy Loop lanes running when one target is sick
persona: Ada
journey: J-bound-runaway-work
expected: Transport failures open only the affected family and target breaker, sick nodes fail fast with target_unavailable, healthy independent lanes keep running, and a successful half-open probe closes the breaker.
entry_points: `compozy loop nodes --state quarantined -o json`; `compozy loop runs show <run-id> -o json`; Loop run events over HTTP/SSE
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-loop-failure-breaker; LP-quarantine-diagnose-requeue
---

QA impact 2026-08-02: Task 04 implements target-scoped transport accounting, fail-fast admission,
and half-open recovery. A real-user walk is blocked until Task 07 exposes breaker transitions and
quarantine inventory through structured CLI/HTTP/UDS surfaces; internal store and coordinator tests
are automated evidence, not a substitute for the public-interface walk required by `qa-execution`.
