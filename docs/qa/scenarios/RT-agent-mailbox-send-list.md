---
id: RT-agent-mailbox-send-list
area: RT
title: Send and list inert lineage messages
persona: Bruno
journey: J-agent-mailbox
expected: An operator message is durably queued or delivered to an allowed lineage session and list/detail reads preserve profile and workspace isolation.
entry_points: compozy message send; compozy message list; messages API; compozy__agent_message
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-follow-up
---

Send to active, busy, and parked child sessions, then compare CLI, HTTP, UDS, and native delivery receipts.
