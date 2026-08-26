# CH-agent-comms-mailbox-backpressure: Flood the mailbox and let its targets die underneath it

```yaml
charter:
  id: CH-agent-comms-mailbox-backpressure
  mission: "As Bruno, push every mailbox brake past its limit, send into children that are working, parked, blocked and expiring, and prove no message is ever silently dropped — every one ends in a receipt that names what happened to it."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-message-a-running-agent
  scenarios: [RT-agent-mailbox-send-list, RT-message-limits-typed-rejections, RT-parked-child-idle-ttl]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Send into each recipient state and read the receipt: a working child gives delivered-into-turn at the next boundary and never mid-tool; an idle one gives woke and consumes a turn on the existing budget substrate; a parked or unreachable one gives queued. Then confirm CLI, HTTP, UDS and the native tool agree on receipt, provenance and durable order — and that none of them renders a message total, a read state, or a seen state, because none exists."
      - "Break each brake in turn and read its code on every surface: rate_limit_per_minute (429, names the window and reset, and a different sender is unaffected), an identical repeat inside dedup_window (409, points at the original id), max_bytes (413, points at the key), and the pending cap. Prove the pending cap counts queued-undelivered transport backlog only — delivered messages must not count toward it."
      - "Push two senders at the same limit concurrently and confirm the checks really run inside the accept transaction: no over-cap row may be committed and then compensated afterwards."
      - "Queue messages for a parked child with a short idle TTL, then let it actually expire. Every queued message must terminalize failed naming the expiry reason BEFORE the target is finalized, so a later list read explains itself instead of showing an eternally queued row. Repeat with a subtree drain and confirm the drain reason attaches the same way."
      - "Prove the clock is honest: idle_expires_at is null while a call is in flight and a timestamp once parked; a child with an open call is never clock-reaped even past its TTL; contact suspends the clock immediately; and park state clears only after a successful wake, not optimistically at the attempt."
      - "Deliver a message containing an embedded command and an approval-looking phrase. It must render provenance-stamped inside a bounded untrusted frame, arrive inert, and be unable to approve a pending permission."
    must_avoid:
      - "Lowering limits in config to make a brake easier to hit and then reporting the shipped behavior — change one key at a time, sequentially against the isolated home, and say which value produced the observation."
      - "Reading delivery from the sender's response alone; the receipt lives on the durable message record and must be re-read after restart."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier. The mailbox is where a durable-first design most easily degrades into best-effort, so
this session owns invariant 13 (rate-limit, dedup-window and pending-cap checks run inside the
message-accept transaction with typed, observable rejections, and the backlog is transport state
rather than read acknowledgment) and invariant 11 (the idle reaper never reaps a session with an open
call; the clock arms only on park; operator-caller sessions are excluded entirely). ADR-003 and
ADR-004 are the decisions under test.

The Garbage Tour is the match: every one of these behaviors is defined by what happens to input the
system does not want — too much, too often, identical, too large, or aimed at something that is about
to stop existing. `RT-parked-child-idle-ttl` sits in this charter rather than in the delegation one
because its user-visible consequence is a mailbox consequence: the reason a queued message failed.
The inert-frame probe belongs here too — a mailbox that can carry an instruction is a privilege
escalation path, and it is cheapest to test while already flooding it.

The cut S3 inbox is worth remembering while walking this: there is deliberately no inbox place, so
every observable in this session must be reachable from CLI, API, or the message's own in-context
turn.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
