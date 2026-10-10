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
5. Stop A before B settles. The watch remains `fired`; resume A and verify one delivery. Repeat
   across daemon restart. Fill A's queue before firing, free a slot and verify retry delivers once.
6. Queue the request behind B's current turn. Stop B: the watch remains `armed`. Cancel or clear the
   unconsumed message: the reply is `dropped`. Supersede a pending steer with requeue: it follows the
   same message until its eventual consuming turn, never the original unrelated turn.
7. Inject dispatch uncertainty without a recorded input. After B settles, stops or the daemon
   restarts, confirm the `unknown` sentence. When an input event exists, its turn's outcome wins.
8. Archive or delete A with an outstanding watch: it is abandoned and no wake is delivered.
   Delete B while its message remains unconsumed: recovery still boots and A receives `unknown`
   with the deleted-session label. Cancel or clear an already delivered wake: no replacement wake
   appears, including after restart.

Task 04 automated evidence covers admission/watch atomicity, legacy replay exclusion, native
receipts in every prompt mode, exact reply content, queue priority, restart, abrupt process loss,
provider/commit faults, pending steering, and the automatic eight-hop reply limit. The canonical
owners are the session reply suite, global admission/queue suite, and daemon subagent integration
suite. Public Web/real-provider QA remains untested until the controller's integrated QA run;
this worker packet prohibits QA labs and full E2E runs.
