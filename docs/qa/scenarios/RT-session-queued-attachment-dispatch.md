---
id: RT-session-queued-attachment-dispatch
area: RT
title: Dispatch queued prompt attachments
persona: Théo
journey: J-session-attachments
expected: A busy-session prompt keeps its attachment refs with the durable queued input and dispatches the same ordered attachments once when the queue entry is delivered; reload and native history/events keep metadata only.
entry_points: HTTP+UDS prompt; native session_prompt; CLI session input; persisted events
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412/03-attachment-queued.png; qa-artifacts/qa/queued-attachment.json
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412.md
overlaps: RT-019
---
