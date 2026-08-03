---
id: LP-durable-wait-restart
area: LP
title: Resume one durable Loop wait across a daemon restart
persona: Ada
journey: J-07
expected: A timer or event wait survives daemon restart, resumes exactly once from its durable row, ignores an event at or before the ahead cursor, consumes a valid ahead arrival once, and preserves workspace isolation throughout recovery.
entry_points: `compozy loop runs show <run-id> -o json`; Loop waiting inventory over CLI/HTTP/UDS/native tools; daemon restart
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-duplicate-event-suppressed
---

acceptance-walk: Park separate timer and event waits, restart the isolated daemon, deliver one event at or behind the stored cursor and one valid ahead event, and inspect the timer due scan. Confirm each wait resumes exactly once, the stale event is ignored, and workspace-scoped native, CLI, and HTTP reads agree after refresh.
