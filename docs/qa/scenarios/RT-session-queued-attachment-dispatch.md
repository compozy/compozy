---
id: RT-session-queued-attachment-dispatch
area: RT
title: Dispatch queued prompt attachments
persona: Théo
journey: J-13
expected: A busy-session prompt keeps its attachment refs with the durable queued input and dispatches the same ordered attachments once when the queue entry is delivered; reload and native history/events keep metadata only.
entry_points: HTTP+UDS prompt; native session_prompt; CLI session input; persisted events
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-019
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
