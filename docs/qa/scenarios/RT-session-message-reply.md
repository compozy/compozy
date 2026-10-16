---
id: RT-session-message-reply
area: RT
title: Receive one durable reply to a session message
persona: Dora
journey: J-session-collaboration
expected: The sender receives one reply for the turn that consumed its message, with durable retry and no redelivery after operator cancellation.
entry_points: compozy__session_prompt; compozy__session_status; compozy session status; session input queue; session history
qa_status: untested
bug_ids:
fix_status:
retest_status: pending
fix_commits:
evidence:
last_report:
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
    and the broken row is logged as `reply_watch.recover_failed`. Armed watches older than 24 hours
    produce a warning. Unrelated transcript turns do not affect normal reconciliation.
11. Long wait: keep B working longer than `session.supervision.quiet_after` + `stop_grace` (shorten
    them for the lab). A, idle with an armed watch, is never stopped for inactivity and shows an
    `awaiting_reply` work signal in `compozy session status A -o json`. Then stop B with the message
    queued: A goes quiet and is stopped for inactivity; resume B and confirm the reply resumes A and
    delivers exactly one wake.

Task 04 automated evidence covers admission/watch atomicity, legacy replay exclusion, native
receipts in every prompt mode, exact reply content, queue priority, restart, abrupt process loss,
provider/commit faults, pending steering, and the automatic eight-hop reply limit. The canonical
owners are the session reply suite, global admission/queue suite, and daemon subagent integration
suite. Public Web/real-provider QA remains untested until the controller's integrated QA run;
this worker packet prohibits QA labs and full E2E runs.


Queue fairness regression (#750)

Keep A in a Goal with a prepared prompt while its current turn is active. Send a watched question
to B, finish B, and inspect A's pending `prompt-reply:` entry. At A's next idle boundary, the reply
wake runs before the prepared Goal and ordinary queued inputs according to its existing priority.
A queued agent message and operator prompt also run before the Goal. Retry the watched send and
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
    "In reply", the hop does not increase, and A's armed watch is released so A is woken once, not
    twice.
13. Set `session.messaging.reply_watch_ttl = "1m"` (restart the daemon), send a watched message to a
    busy B and wait past the TTL: A receives one wake with the `expired` outcome, the sent card reads
    "No reply" and the body "No reply arrived in time. The message may still be pending; check the
    target session." With the default `0s` an armed watch never expires and keeps A out of inactivity
    stop while B is live.
