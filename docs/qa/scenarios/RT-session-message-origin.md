---
id: RT-session-message-origin
area: RT
title: Attribute session messages across delivery and restart
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: Native agent messages retain daemon-stamped origin through direct, queue, steer, interrupt and restart; provider text has the sender header while stored text does not; replay is actor-scoped, self-targeting and hop nine are rejected, and agent-authored queue entries can only be canceled.
entry_points: compozy__session_prompt; session input list/replace/promote/cancel; session events; HTTP and UDS prompt/queue and events
qa_status: untested
bug_ids: none
fix_status: not-applicable
retest_status: pending
fix_commits: pending
evidence: internal/daemon/subagent_integration_test.go TestSessionMessageOriginDaemonIntegration; internal/session/manager_busy_input_test.go TestManagerPromptOriginDelivery and TestManagerPromptOriginHopFence
last_report: pending tail QA
overlaps: RT-session-message-reply; ET-web-session-message-card
---

Spec owner: `.compozy/tasks/agent-collaboration/_spec.md`, task 02, business rules 1–5 and 17–18.
Automated ownership: UT-001–013 (origin portion), UT-015–018; IT-001–004, IT-020/024, IT-029/030.
The packet's scoped ACP/SQLite checks cover backend delivery; real-provider and Web walks belong to
the trailing QA pair. Do not infer a real-provider pass from fixture evidence.

1. From session A, call the native message tool targeting idle session B with a fresh message and
   idempotency key. Inspect B's event: origin names A and A's workspace, hop is 1, and text is exactly
   the message. The provider receives the sender header. UI metadata exposes `origin` separately.
2. Repeat while B is busy for queue, steer and interrupt. Inspect the queued origin before dispatch
   and compare it with the stored input event after dispatch. The provider header must survive every
   route, including steer fallback.
3. Queue a message, restart before dispatch, then resume B. Verify one delivery with the same origin.
4. Send an operator prompt through HTTP and CLI. Its queue/event/UI metadata has no origin.
5. Retry the same agent message after renaming A: original admission and title remain. Retry from
   another agent, from the operator, or with a changed notification flag: identity conflict.
6. Replace or promote an agent-authored queue entry: 409 `input_agent_authored` and unchanged text.
   Cancel the same entry successfully. Reply-watch effects are owned by the reply scenario.
7. Send to A from A: `invalid_request`, no admission. Build a chain to hop 8, including steers during
   a turn: the next send returns `message_hop_limit`. An operator turn starts at hop 0 again.
8. Upgrade a database containing v4 receipts and queue rows. Values survive and `origin_json` starts
   NULL. Replaying the original request returns the original result without fabricated attribution.

Review regression: while a hop-7 or hop-8 steer has reached the provider but its input receipt is
blocked, send from the receiving turn. The outgoing message has hop 8 for the hop-7 input; the
hop-8 input refuses the ninth hop. A failed injection must not lower the current-turn hop floor.
Automated owner: `TestSessionMessageOriginResponseBeforeReceiptIntegration` (IT-029) and
`TestManagerPromptOriginHopFence` (UT-016).
