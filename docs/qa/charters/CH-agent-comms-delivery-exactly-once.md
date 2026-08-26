# CH-agent-comms-delivery-exactly-once: Crash the daemon around every delivery and count the wakes

```yaml
charter:
  id: CH-agent-comms-delivery-exactly-once
  mission: "As Ada, settle calls while killing the daemon in the window between commit and notify, fan out batches across restarts, and prove every caller receives exactly one result-carrying wake per call — never two, never zero, and never a completed signal without its result."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-delegate-work-to-an-agent
  scenarios: [RT-call-wake-delivery-exactly-once, RT-agent-call-follow-up, RT-agent-call-batch]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Kill the daemon between settlement commit and wake notification, repeatedly, until the window is genuinely hit. On boot the caller must receive the wake from the durable row, and durable wake_event_id dedupe must make it exactly one. Then attach two watchers to the same call — an await from one client and the caller's own turn — and confirm they observe one delivery, not one each."
      - "Walk all nine terminal states and read the wake text the daemon actually emits, not a paraphrase. Completed carries a valid result reference and bounded preview; failed, canceled, timeout, expired, invalid-result and completed-without-result carry the typed reason instead. A completed signal without its result must not be producible on any path."
      - "Fill max_active_per_root, then let an already-admitted call settle. The wake still arrives — a committed call is never admission-denied. Confirm there is no delivery-skip event in the catalog to find, and that its absence is the evidence."
      - "Prove queued execution lives only in task_runs: every child-starting call carries a call_activation run claimed through the task authority, the calls tables carry no claim or lease columns, and nothing scans call rows to start work. Then fail an activation deliberately — the call must settle failed with a typed spawn reason rather than disappear between commit and start."
      - "Fan out a mixed batch (two valid agents, one unknown), restart mid-flight, and confirm no lane duplicates and no lane is lost. Then complete a call, let the child park, call its session id, and confirm the follow-up revives the same child with context intact, mints its own call id, and that the shared child_session_id still renders one subtree rather than two."
    must_avoid:
      - "Counting wakes from a live UI; count them in the caller's durable transcript and in the event stream, because a browser can drop or replay independently of the runtime."
      - "Accepting a batch response as proof of activation — an accepted item is a committed row, and this session is about what happens after commit."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier. This is the delivery half of the settlement hot spot and it owns invariants 3
(delivery row written in the settlement transaction; notify only after commit; a committed
completion is never admission-denied), 3a (admission and activation are separate phases, with no
subprocess work inside a store transaction), 3b (queued execution lives only in `task_runs`), 4 (the
wake carries the call identity, terminal state and exactly the applicable payload) and 12 (durable
`wake_event_id` dedupe makes redelivery idempotent). ADR-004 and ADR-011 are the decisions under
test: hybrid cost semantics with durable delivery always, and accounting-only activations with no
Network-Live-style admission bounds.

Ada is the right persona because the observable is structured — the wake as it reaches a caller's
turn and the event record behind it — not a rendered surface. The Multi-Tab Tour maps onto the real
failure mode better than Garbage would: exactly-once breaks when more than one observer or more than
one process boundary is involved, which is precisely what a restart plus a second waiter creates.
Lessons L-003 and L-005 are why `RT-call-wake-delivery-exactly-once` insists on proving nothing scans
call rows to start work; regressing that would reintroduce a second work queue beside `task_runs`.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
