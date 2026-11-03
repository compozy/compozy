---
id: RT-session-message-origin
area: RT
title: Attribute session messages across delivery and restart
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: Native agent messages retain daemon-stamped origin through direct, queue, steer, interrupt and restart; provider text has the sender header while stored text does not; replay is actor-scoped, self-targeting and hop nine are rejected, and agent-authored queue entries can be canceled or promoted but not replaced.
entry_points: compozy__session_prompt; session input list/replace/promote/cancel; session events; HTTP and UDS prompt/queue and events
qa_status: partial
bug_ids: none
fix_status: not-applicable
retest_status: pass
fix_commits: pending
evidence: internal/daemon/subagent_integration_test.go TestSessionMessageOriginDaemonIntegration and TestReplyWatchNativeIntegration (output schema, #1186); internal/session/manager_busy_input_test.go TestManagerPromptOriginDelivery and TestManagerPromptOriginHopFence; docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/ (2026-11-03 stays-on QA walk)
last_report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md
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
6. Replace an agent-authored queue entry: 409 `input_agent_authored` and unchanged text. Promote it
   to steer successfully (step 13). Cancel the same entry successfully. Reply-watch effects are owned by the reply scenario.
7. Send to A from A: `invalid_request`, no admission. Build a chain to hop 8, including steers during
   a turn: the next send returns `message_hop_limit`. An operator turn starts at hop 0 again.
8. Upgrade a database containing v4 receipts and queue rows. Values survive and `origin_json` starts
   NULL. Replaying the original request returns the original result without fabricated attribution.

Review regression: while a hop-7 or hop-8 steer has reached the provider but its input receipt is
blocked, send from the receiving turn. The outgoing message has hop 8 for the hop-7 input; the
hop-8 input refuses the ninth hop. A failed injection must not lower the current-turn hop floor.
Automated owner: `TestSessionMessageOriginResponseBeforeReceiptIntegration` (IT-029) and
`TestManagerPromptOriginHopFence` (UT-016).


Queue fairness regression (#750)

Queue an agent message and an operator prompt while a Goal prompt is already prepared behind
an active turn. At that turn's boundary, both public inputs run before the Goal; the agent input
retains its sender origin and header, and the operator input has no origin. Queue positions exclude
internal Goal prompts. Repeat with a prepared continuation and after restart. Queue mode waits for
the active turn; fenced steer and interrupt retain their existing delivery behavior.

Automated owner: `TestLoopGoalManagedRuntimeIntegration/Should_drain_messages_and_reply_wakes_before_a_prepared_Goal_at_the_turn_boundary`
(real SQLite, admission, reply-watch service and managed session selector). Full public UI and
real-provider walks remain delegated to CI/integrated QA per maintainer instruction.

The first public claim freezes a finite admission batch before the Goal. Add more messages during
that batch and reopen the store: the Goal runs after the original batch, before later arrivals.
Store owner: `TestGlobalDBSessionInputQueueGeneration/Should_preserve_a_bounded_Goal_input_batch_across_new_arrivals_and_reopen`.

Busy-target policy regression (#760)

9. Human-first order: with B busy, queue a human prompt and several agent messages and a reply wake.
   The queue list shows the human entry first with `queue_position` equal to its rank; agent steers
   sent afterwards never jump it. Fill B with agent messages up to `queue_cap`: a human send still
   succeeds (separate caps).
10. An agent `mode: "steer"` to a provider without injection stays queued (no interrupt, active turn
    untouched). An agent `mode: "interrupt"` is delivered as steer and the result reports `steer`.
    A queued human steer survives a later agent steer. With a human prompt queued on an injecting
    provider, an agent `steer` is not injected: it reports `interrupt_fallback` with `after_turn`
    delivery and runs after the human prompt.
11. Send three status messages with `supersede: true` from A to B: only the latest remains queued and
    `canceled_queued_entries` counts the replaced ones; another sender's and the human's entries stay.
    A superseding `steer` injected into B also cancels A's older queued messages and reports them in
    `canceled_queued_entries`; they never run after the turn.
12. Eight consecutive queued agent messages and reply wakes dispatch as one turn: each keeps its own
    origin event and reply watch; a human entry is never merged.
13. Promotion: keep B in one turn past `session.messaging.queue_promote_after` (shorten it for the lab).
    A queued agent message is injected as steer with its original message id; with `0s` it keeps
    waiting; with a human entry waiting it is not promoted. As the operator, Steer an agent queue row
    (works) and try to replace it (409 `input_agent_authored`).
    Fail persistence while a pending injection is rejected: status and Stop remain responsive,
    the entry stays leased, and restoring storage returns it to after-turn delivery without
    injecting the guidance again. Also lose the acknowledgement after an input write commits:
    history retains exactly one authored event and the queue entry still settles as sent.
14. Hop and rate: A answering the session that drives its turn, or sending with `in_reply_to`, keeps
    the hop; forwarding to a third session adds one. Exceed `rate_limit_messages` within the window:
    that send returns `message_rate_limited` with `limit`, `window` and `retry_after_seconds`, and both
    HTTP (429) and the native tool carry the same message: "Message rate limit reached: <limit>
    messages to session <id> within <window>. Retry after <d>, and send fewer, milestone-level
    messages." Other sends and sessions are unaffected; `-1` disables the budget.
15. CLI origin: inside a managed session run `compozy session prompt <id> "hi"`; it stays operator
    input and stderr prints the `--from-session auto` hint. With `--from-session auto` the receiver sees
    A as sender, `--in-reply-to` links the card, and `--supersede` replaces the older queued prompt.
    `--from-session auto` outside a managed session is refused, and so are `--in-reply-to` and
    `--supersede` without `--from-session` (HTTP and the native tool return `invalid_request` for a
    caller without an agent session).
16. Web: the queue strip shows "From {title}" with Steer and Remove; consecutive inbound cards from one
    sender collapse into "{title} · N messages"; a reply shows the "In reply" chip; a session in
    Needs attention shows its reason.

## Stays-on peer addressing and pause

1. List reachable peers with `agent instance list --peers`; a denied workspace contributes neither a
   row nor a private count. Keep the cursor bound to the same caller and filters.
2. Rotate a peer and send from an active work turn using its agent name and workspace. The receipt's
   `target_session_id` names the successor; the original session receives no new input.
3. Pause that peer, send again with `wait:true`, and inspect `delivery: held` and queue `held: true`.
   Restart the daemon, confirm the row remains held, then resume as operator. It is delivered once.
4. A human prompt on the paused main runs without resuming autonomy; a check cannot call the message
   tool, and a missing name produces `agent_instance_not_found`.
5. From a stays-on main, message a fresh idle child it created (`--parent`), with `runtime` set. The
   receipt reports `delivery: queued` and `target_session_id`, and the caller's MCP client accepts it:
   no "does not match the tool's output schema" error and no "Could not send" card (#1186). Validate
   the `structuredContent` against the advertised output schema for direct, queued and held receipts.

Automated backend owners: `TestAgentInstancePeers`, `TestGlobalDBSessionPromptAdmission`,
`TestInstanceContextExhaustionIntegration` and `TestInstancePausedRuntimeIntegration`.
The real ACP journeys cover successor delivery and restart retention.

QA verification 2026-11-03 (stays-on peer addressing, soh/qa 583415ef5, isolated acpmock lab): Partial. `agent instance list --peers` returned name/description/scope/state rows. A native `compozy__session_prompt` with `agent: release-watch`, sent while it was paused, returned `delivery: held`. The queue row was `held: true` and stayed held with `unattended_turns` unchanged across a daemon restart. An operator resume delivered it in one turn (`turn-2b4a49e90d7d9360`), and a second resume added no prompt (`restart-retention.txt`). A human prompt on a paused main ran without resuming autonomy. A missing name returned code `agent_instance_not_found`, but the message is "Internal Server Error" (paper cut). Not walked: delivery to a rotated peer's successor, the denied-workspace peer filter, and a check calling the message tool.

QA verification 2026-11-03 (#1186, isolated acpmock lab on the fix branch): Pass for step 5. A stays-on main sent `compozy__session_prompt` with `runtime` to its fresh idle child through the hosted MCP proxy and got `delivery: queued` with `target_session_id`. The MCP TypeScript SDK's AJV validator accepted that `structuredContent` against the advertised schema; the same receipt against the previous schema reproduced the reported error verbatim. Held and direct receipts are owned by `TestInstancePausedRuntimeIntegration` and `TestReplyWatchNativeIntegration`.
