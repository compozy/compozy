---
id: TA-task-wake-dedup
area: TA
title: Deliver one creator wake after cache eviction
persona: Ada
journey: J-operate-bounded-task-capacity
expected: A repeated task creator wake_event_id delivers once after cache eviction or daemon restart even with a large unrelated event history; the authoritative lookup is task-scoped and never suppresses a different task or wake identity.
entry_points: task creator session; task event ledger over CLI/HTTP/UDS; daemon restart
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-016; TA-parent-rollup-completion; LP-task-rollup-wakes-loop
---

Added by the Hermes comparison wake-dedup milestone. Exercise delivered and suppressed wake audit
rows, cache eviction, restart, decoy event types, another task with the same identity, and a distinct
wake identity on the same task.

Flag only: the later Hermes comparison QA cycle owns execution and evidence.

Phase C planning 2026-07-19: linked to J-operate-bounded-task-capacity; companion to US-016 EC-1
(indexed wake dedup, cost independent of event count). Forensic contract (SD-006): timestamped
commands with observed output for the delivered and suppressed wake audit rows across cache
eviction and restart, plus the decoy-identity non-suppression checks.
