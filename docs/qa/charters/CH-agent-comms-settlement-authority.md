# CH-agent-comms-settlement-authority: Race the settlement writer and prove exactly one terminal survives

```yaml
charter:
  id: CH-agent-comms-settlement-authority
  mission: "As Bruno, drive a call toward two terminals at once — return against cancel, cancel against deadline, a repair round against a crash — and prove the single settlement writer produces exactly one terminal state with a valid result, never a half-settled call and never a mutated one."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-delegate-work-to-an-agent
  scenarios: [RT-agent-call-golden-path, RT-agent-call-cancel, RT-agent-call-deadline-timeout, RT-call-return-contract-repair]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Walk create → await → result clean first, in both Global scope and a workspace, so the baseline is real before anything is broken. Then replay the same idempotency key, fire two concurrent creates carrying it, and replay it once more with a different result budget: one call row, replayed true, then call_idempotency_conflict naming the original id."
      - "Kill the daemon inside the return transaction. The call must come back either untouched or wholly settled — a completed state row with a missing or invalid result reference must be impossible to produce. Repeat until the window is genuinely hit rather than assumed."
      - "Race return against cancel, and cancel against an opt-in deadline, in both orders. Each pair must resolve to exactly one terminal, and the loser must appear only as superseded evidence — confirm the terminal itself was never rewritten. Cancel a call whose activation run is still unclaimed and one whose claim already won: the first is fenced out through the task authority, the second falls back to managed stop."
      - "Fail the contract twice on purpose. The first failure must return the validator's errors verbatim from the already-sanitized payload and grant exactly one repair attempt; the second settles invalid-result with both attempts readable. Then prove an infrastructure failure does not consume the attempt, and that a single-key wrapper around a valid payload is unwrapped rather than failed."
      - "Separate the two clocks that both say timeout: let an await box elapse (exit code 3, resume handle, call still running) and let a call deadline elapse (settled timeout by the sweeper). Confirm a call created without --deadline has none, and that over-max timeouts clamp to 30 minutes and say so instead of rejecting."
    must_avoid:
      - "Reading state from the response that performed the action — re-read from compozy call show or the HTTP record after every race, because the point is which write won, not what the client was told."
      - "Treating a repeated cancel on a terminal call as an error; it is an idempotent success, and call_already_settled belongs to return, not cancel."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier, first session — highest-impact journey crossed with the highest-blast-radius tour,
run while attention is fresh. This charter owns the invariants that everything else in the cycle
assumes: 1 (settlement methods are the only writers of terminal state, fenced by actor), 2
(sanitize, validate, blob write and terminal write are one transaction, so a contracted `completed`
row without a valid result reference cannot exist), 5 (post-terminal writes are rejected; late
outcomes land only in `superseded_ref`), 6 (the idempotency uniqueness fence is in the database, so
concurrent duplicates resolve to one row), 3c (terminalizing always fences the activation run first)
and 3e (the deadline sweeper is the single authority for `timeout`). It carries ADR-001, ADR-009 and
ADR-013 with it.

The Interrupt Tour is the only honest lens here. Every one of these invariants is a claim about what
happens when two things arrive at once or a process dies mid-write; a Feature Tour would walk the
happy path and confirm nothing. `BUG-20260729-accepted-start-stop-identity-race` — an immediate stop
that split session-creation identity — is the closest prior art in the registry, and it is the shape
of failure this session is hunting one layer down, in the call domain rather than the session one.
`RT-agent-call-golden-path` rides along because a race result means nothing without a proven
baseline, and because it is where the idempotency fence is reachable.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
