---
id: RT-in-context-call-messages
area: RT
title: Calls and messages read as turns in the conversation
persona: Ada
journey: J-supervise-delegation-trees
expected: A child session shows the ask that started it, the mailbox messages it received with provenance stamps and delivery receipts inside an inert untrusted frame, and the completion wake carrying the daemon's own wake line verbatim. A caller's turn shows its compozy__agent_call as a call card, and a batch as one fan-out card. No read or seen state renders anywhere.
entry_points: web /agents/reviewer/sessions/ses_01JBD8G2MZTX transcript; HTTP and UDS GET /api/workspaces/{workspace_id}/messages?session=ses_01JBD8G2MZTX&limit=25; the provenance-stamped synthetic turn metadata for message_id msg_01JBD8M2R4V7
qa_status: pass
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-agent-mailbox-send-list; RT-call-wake-delivery-exactly-once; RT-session-calls-inspector-panel
---

Added by task_06. The walk must confirm the transcript order is the daemon's durable order and that an embedded command inside a message body stays inert.

The mailbox has no place of its own — this is its home, so the receipt has to be visible here and
update in place from queued to delivered or woke on the same record. Confirm the wake line is the
daemon's own text rather than a rewritten paraphrase, that no read or seen state renders anywhere,
and that the untrusted frame holds: an embedded command must not be able to approve a pending
permission from inside a message body.
