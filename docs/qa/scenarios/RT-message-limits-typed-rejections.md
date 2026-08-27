---
id: RT-message-limits-typed-rejections
area: RT
title: Refuse over-limit messages inside the accept transaction
persona: Bruno
journey: J-message-a-running-agent
expected: Rate limit, dedup window, pending cap and byte ceiling are all checked inside the message-accept transaction and reject with their own typed, observable codes, and the pending cap counts queued-undelivered transport backlog rather than anything resembling read state.
entry_points: compozy message send ses_01JBD8G2MZTX "bounded message"; HTTP and UDS POST /api/workspaces/{workspace_id}/messages with {"to":{"session_id":"ses_01JBD8G2MZTX"},"text":"bounded message"}; compozy__agent_message with {"to":"parent","text":"bounded message"}; compozy config get calls.messages.rate_limit_per_minute and compozy config set calls.messages.rate_limit_per_minute 1, repeated for calls.messages.dedup_window, calls.messages.pending_cap, and calls.messages.max_bytes
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-agent-mailbox-send-list; RT-calls-config-effects
---

The mailbox has four brakes and they all live inside the accept transaction, which is what makes
their rejections trustworthy rather than best-effort. Push each one and read its code on every
surface — CLI exit status, HTTP status, and the native tool result must agree.

Exceed `rate_limit_per_minute` for one sender and confirm `message_rate_limited` (HTTP 429) names the
window and its reset, then confirm a different sender is unaffected. Repeat an identical message
inside `dedup_window` and confirm `message_duplicate` (HTTP 409) points at the original message id
rather than silently accepting a second row. Exceed `max_bytes` and confirm `message_too_large`
(HTTP 413) points at the configured key.

The pending cap deserves the most care because it is the one most easily misread. Fill a recipient's
**queued-undelivered** backlog to `pending_cap` and confirm the rejection is typed and observable —
and confirm what the cap counts: transport state only. Delivered messages must not count toward it,
and there must be no read or seen semantics anywhere near it. Then let the backlog drain and confirm
sending works again.

Finally push the same limits concurrently from two senders and confirm the transaction boundary
holds — no over-cap row is committed and then compensated after the fact.
