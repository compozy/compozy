---
id: RT-agent-mailbox-send-list
area: RT
title: Send and list inert lineage messages
persona: Bruno
journey: J-message-a-running-agent
expected: An operator message is durably queued or delivered to an allowed lineage session and list/detail reads preserve profile and workspace isolation.
entry_points: compozy message send ses_01JBD8G2MZTX "Prioritize the loop package first"; compozy message list --session ses_01JBD8G2MZTX --limit 2; HTTP and UDS POST /api/workspaces/{workspace_id}/messages with {"to":{"session_id":"ses_01JBD8G2MZTX"},"text":"Prioritize the loop package first"}; HTTP and UDS GET /api/workspaces/{workspace_id}/messages?session=ses_01JBD8G2MZTX&limit=2; compozy__agent_message with {"to":"parent","text":"Blocked on the loop tests"} and with {"to":"ses_01JBD8G2MZTX","text":"Continue"}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-message-limits-typed-rejections; RT-parked-child-idle-ttl; RT-in-context-call-messages
---

Send to active, busy, and parked child sessions, then compare CLI, HTTP, UDS, and native delivery receipts.

The four receipts — `delivered-into-turn`, `woke`, `queued`, `failed` — are the only truth on offer,
because the runtime models no read or seen state. Confirm no surface renders one, and none renders a
message total either. Delivery must never happen mid-tool: a message to a working child lands at the
next boundary, non-interrupting and at **no extra turn cost**. A message to an idle or parked child
wakes it, and that wake **is** a billable turn accounted through the existing wake and budget
substrate. Check the cost side of both, not just the receipt — the two paths are deliberately priced
differently, so a boundary injection that quietly bills a turn is a finding even though its receipt
looks right. Targets outside the lineage or grant must fail `call_target_denied`, and a target
awaiting a human decision must fail `message_target_blocked` pointing at the decision surface rather
than at messaging.
