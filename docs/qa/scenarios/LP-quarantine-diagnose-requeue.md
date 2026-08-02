---
id: LP-quarantine-diagnose-requeue
area: LP
title: Diagnose, repair, and requeue a quarantined Loop node
persona: Ada
journey: J-bound-runaway-work
expected: A repeated failing node parks with sanitized bounded repair context while independent work continues, required consumers name the quarantined dependency, and requeue records provenance before normal bounded succession completes the run.
entry_points: `compozy loop nodes --state quarantined -o json`; `compozy loop node requeue <node-id> --run-id <run-id> -o json`; Loop run events over HTTP/SSE
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-sick-target-degrades-one-lane; TA-loop-failure-breaker
---

QA impact 2026-08-02: Task 04 implements atomic quarantine, dependency attention, provenance,
epoch fencing, and `requeue`-origin succession. A real-user walk is blocked until Task 07 ships the
public inventory and requeue verbs with event parity. Task 08 will add the Web surface, but it is not
required for the first structured CLI/HTTP/UDS walk.
