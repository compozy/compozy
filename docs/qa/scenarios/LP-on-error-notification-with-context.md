---
id: LP-on-error-notification-with-context
area: LP
title: Deliver an on-error notification with the committed failure context
persona: Loop operator
journey:
expected: A declared on_error effect is delivered after the node failure commits, carries the exact node, attempt, disposition, failure, and run link, and records an isolated effect result without changing the node outcome.
entry_points: Loop definition; loop run event stream
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

QA impact 2026-08-02: Task 03 implements the transactional outbox and daemon relay. A real-user walk is blocked until Task 07 adds `effect_results` and `custom_event` to the public `LoopRunEventKind` contract and SSE parity, providing the independent public read path required by `qa-execution`.
