---
id: LP-durable-wait-restart
area: LP
title: Resume one durable Loop wait across a daemon restart
persona: Ada
journey: J-07
expected: A timer or event wait survives daemon restart, resumes exactly once from its durable row, ignores an event at or before the ahead cursor, consumes a valid ahead arrival once, and preserves workspace isolation throughout recovery.
entry_points: `compozy loop runs show <run-id> -o json`; Loop waiting inventory over CLI/HTTP/UDS/native tools; daemon restart
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-duplicate-event-suppressed
---

QA impact 2026-08-02: Task 06 implements durable timer/event wait rows, restart-safe due scans,
the event observer bridge, ahead-arrival policy, and atomic `ResumeWait`. A real-user restart walk
is blocked until Task 07 exposes the waiting read model and resume surface; Task 13 owns the
isolated restart walk.
