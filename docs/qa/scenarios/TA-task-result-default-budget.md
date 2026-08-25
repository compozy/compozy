---
id: TA-task-result-default-budget
area: TA
title: Retain an uncontracted task result above 64 KiB
persona: Ada
journey: J-operate-bounded-task-capacity
expected: A task result larger than 64 KiB and no larger than the configured 256 KiB default budget completes successfully, retains the exact result bytes, and remains readable after daemon restart.
entry_points: task run completion over CLI/HTTP/UDS; task run read; daemon restart
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Flagged by the agent communications legacy-contract adoption. Exercise both sides of the old
64 KiB boundary and the configured default-budget boundary, including exact-byte verification
after restart. The agent communications QA phase owns execution and evidence.

src: .compozy/tasks/agent-comms/task_02.md
