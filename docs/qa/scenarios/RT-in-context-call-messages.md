---
id: RT-in-context-call-messages
area: RT
title: Calls and messages read as turns in the conversation
persona: Ada
journey: J-08-watch-and-maintain
expected: A child session shows the ask that started it, the mailbox messages it received with provenance stamps and delivery receipts inside an inert untrusted frame, and the completion wake carrying the daemon's own wake line verbatim. A caller's turn shows its compozy__agent_call as a call card, and a batch as one fan-out card. No read or seen state renders anywhere.
entry_points: web session transcript; GET /api/workspaces/{id}/messages?session=; synthetic turn metadata
qa_status: untested
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: 
overlaps: 
---

Added by task_06. The walk must confirm the transcript order is the daemon's durable order and that an embedded command inside a message body stays inert.
