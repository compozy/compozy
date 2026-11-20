---
id: RT-session-message-reply
area: RT
title: Receive durable replies to a session message
persona: Dora
journey: J-session-collaboration
expected: The sender receives the consuming turn reply and one follow-up if work is still pending. Both survive restart without duplicate delivery.
entry_points: compozy__session_prompt; compozy__session_status; compozy session status; session input queue; session history
qa_status: pass
bug_ids:
fix_status:
retest_status: pending
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-31-issue-1102-cross-workspace-reply.md
overlaps: RT-session-prompt-cancel; RT-session-spawn-wake
---

1. Start sessions A and B. From A, send B a message with `notify_on_complete: true`; retain
   `message_id` and `reply_watch.id`. End A's current turn.
2. Check A's native, HTTP and CLI status. Its `reply_watches` entry is `armed`, names B and the
   message, and is absent from an unrelated sender or workspace.
3. Complete B's consuming turn. A receives exactly one `session_reply` wake with the last non-empty
   assistant text of that turn. Nested tool output and later turns contribute no text. Status no
   longer lists the delivered watch.
4. Repeat with B failing, canceling, and producing empty output. Confirm `failed` with its error,
   `canceled`, and `(no reply text)`. Produce more than 12000 Unicode characters and confirm the
   exact rune bound plus the history pointer.
5. Stop A yourself before B settles. The watch remains `fired`; resume A and verify one delivery.
   Repeat across daemon restart: A is resumed by the reply and receives one wake. Fill A's queue
   before firing, free a slot and verify retry delivers once.
6. Queue the request behind B's current turn. Stop B: the watch remains `armed`. Cancel or clear the
   unconsumed message: the reply is `dropped`. Supersede a pending steer with requeue: it follows the
   same message until its eventual consuming turn, never the original unrelated turn.
7. Inject dispatch uncertainty without a recorded input. After B settles, stops or the daemon
   restarts, confirm the `unknown` sentence. When an input event exists, its turn's outcome wins.
8. Archive or delete A with an outstanding watch: it is abandoned and no wake is delivered.
   Delete or archive B while its message remains unconsumed: A receives `unknown` immediately,
   without a daemon restart (deletion uses the deleted-session label). Cancel or clear an already delivered wake: no replacement wake
   appears, including after restart.

9. Fail a send before dispatch commit, then retry with the same message ID and idempotency key.
   The same watch is re-armed, the result reports its persisted state, and the eventual answer is
   delivered once. Activation refusal returns the send error without a second `dropped` wake.
10. Recover a broken watch alongside a healthy one. Boot completes, the healthy watch reconciles,
    and the broken row is logged as `reply_watch.reconcile_failed` with its watch, sender and target IDs.
    Armed watches older than the configured TTL produce an `expired` wake. Unrelated transcript turns do not affect normal reconciliation.
11. Long wait: keep B working longer than `session.supervision.quiet_after` + `stop_grace` (shorten
    them for the lab). A, idle with an armed watch, is never stopped for inactivity and shows an
    `awaiting_reply` work signal in `compozy session status A -o json`. Then stop B with the message
    queued: A, idle with no turn in flight, stays `active` and is never stopped for inactivity (with
    a short `idle_release_after` its runtime is released: `runtime.status: "unbound"`, `idle`
    badge); resume B and confirm the reply wakes A exactly once, rebinding its runtime if it was
    released.

Task 04 automated evidence covers admission/watch atomicity, legacy replay exclusion, native
receipts in every prompt mode, exact reply content, queue priority, restart, abrupt process loss,
provider/commit faults, pending steering, restored sender hops across nine sequential replies,
and the independent eight-hop forwarding limit. The canonical
owners are the session reply suite, global admission/queue suite, and daemon subagent integration
suite. Public Web/real-provider QA remains untested until the controller's integrated QA run;
this worker packet prohibits QA labs and full E2E runs.


Queue fairness regression (#750)

Keep A in a Goal with a prepared prompt while its current turn is active. Send a watched question
to B, finish B, and inspect A's pending `prompt-reply:` entry. At A's next idle boundary, a queued
operator prompt runs first; a queued agent message and the reply wake then run together as one
batched turn (#760), all before the prepared Goal. Retry the watched send and
wake delivery: exactly one wake runs. The Goal then resumes with its binding and turn limit intact;
only its own dispatched work increments its work-turn count. Repeat across restart.

Automated owner: `TestLoopGoalManagedRuntimeIntegration/Should_drain_messages_and_reply_wakes_before_a_prepared_Goal_at_the_turn_boundary`
(real SQLite, admission, reply-watch service and managed session selector). Full public UI and
real-provider walks remain delegated to CI/integrated QA per maintainer instruction.

The first public claim freezes a finite admission batch before the Goal. Add more messages during
that batch and reopen the store: the Goal runs after the original batch, before later arrivals.
Store owner: `TestGlobalDBSessionInputQueueGeneration/Should_preserve_a_bounded_Goal_input_batch_across_new_arrivals_and_reopen`.

Explicit reply and expiry regression (#760)

12. B answers A with `compozy__session_prompt` and `in_reply_to` set to A's message id: A's card shows
    "In reply", the hop does not increase, and A's armed watch stays armed: when B's turn ends A still
    receives the completion wake, so an early acknowledgement never cancels it.
13. The default `session.messaging.reply_watch_ttl` is `24h`; for the lab, override it to `"1m"`
    (restart the daemon), send a watched message to a busy B and wait past the TTL: A receives one wake
    with the `expired` outcome, the sent card reads "No reply" and the body "No reply arrived in time.
    The message may still be pending; check the target session." After expiry the watch no longer
    keeps A out of inactivity stop. With `"0s"` an armed watch never expires and keeps A out of
    inactivity stop while B is live.


## Interim replies and periodic recovery

14. Have B register a terminal or timer monitor, then end the consuming turn. A receives an interim
    reply that names the monitor. Check that A's `reply_watches` and `awaiting_reply` still show the
    original watch. Leave another monitor open in B's next turn and confirm A receives no wake.
15. On B's last monitor wake, check its provider prompt for the outstanding-reply header. Finish B's
    work and confirm A receives exactly one follow-up with that turn's last answer and outcome.
    Repeat after restarting the daemon while the watch is outstanding, and verify the watch stays
    armed while the recovered monitor is active. Repeat with an operator-started turn that settles
    with no awaited work left and confirm its answer is the follow-up. Repeat with B stopped or
    deleted during the wait, and with B's final turn failing.
16. Repeat with B waiting only for a reply from A. Verify the first reply settles without a
    follow-up. Send two watched requests from A to B and let B consume them in separate turns
    while a monitor remains active. Verify A receives both answers and only the newer watch sends
    a follow-up. Repeat with a one-minute TTL and verify the original deadline expires the
    outstanding follow-up.
17. Skip the event-driven settle hook in the service harness. With no active turn, queued message,
    or wake-producing work, a periodic pass delivers a readable reply. A settled but unreadable
    result delivers `unknown` after one grace interval. It says the reply could not be read if
    the message was consumed. A persisted MCP reload must leave the readable answer intact.
18. Cancel B's monitor before another turn settles. Verify the follow-up says the target settled
    without a further answer and the interim reply stands.
19. Have B register a short timer inside the consuming turn and keep that turn active until the
    timer wake is queued. Verify A receives an interim reply that names the pending monitor wake,
    then exactly one follow-up with the answer from the timer's turn.

Service-level coverage lives in `TestReplyWatchPeriodicSettlement` and `TestReplyWatchFollowUp`.
Boot ordering is covered by `TestBootRecoversReplyWatchAfterMonitors`.
The live provider and Web walkthroughs for these additions are pending.

Release integration verification (2026-10-18): the native busy-sender case keeps the operator input
ahead of the reply wake, waits for dispatch before claiming delivery, and checks one reply with the
original sender hop after restart. `TestReplyWatchNativeChainIntegration` checks nine delivered
watches, nine target inputs and nine wakes with matching identities.

QA impact 2026-10-23 (harness dogfood): a truncated reply points to its actual target session
and reply turn with `type: "agent_message"`. Read all chronological answer chunks using the
existing history cursors; if the answer itself is offloaded, follow artifact pages through EOF.
This changes recovery guidance only, not reply-watch admission or delivery semantics. The
native real-SQLite history suite verifies complete filtered answers and cursor direction;
[focused verification](../reports/2026-10-23-harness-dogfood.md) distinguishes this from a new
provider-backed recipient delivery walk.

## Cross-workspace explicit reply

With two isolated workspaces in one profile, let A send B `ping` through an authorized
cross-workspace call. From B, send A `pong` with `in_reply_to` set to the received message ID.
Verify delivery both with omitted workspace and with A's explicit workspace. Repeat with an
unknown message ID, a message from a different peer, no reply ID, a different profile, an
attachment, and a runtime override: none acquires access through the reply exception. Confirm
B still cannot read A's status/history without ordinary workspace authorization.

In B's transcript, verify the Sent card shows A's current title and workspace, including an
older failed send without `target_workspace_id`. The failed card must preserve its actual
permission error. The project reply and Web label walk is verified in the linked 2026-10-31 report; additional authority variants have owning-suite coverage.

QA verification 2026-10-31 (#1102): the targeted isolated project ping/pong, five authorization denials, and live From/Sent/error labels after reload passed. See [scoped report](../reports/2026-10-31-issue-1102-cross-workspace-reply.md) for evidence and coverage limits.
