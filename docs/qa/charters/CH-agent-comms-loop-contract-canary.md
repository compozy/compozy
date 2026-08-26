# CH-agent-comms-loop-contract-canary: Run a Loop that was never supposed to notice this change

```yaml
charter:
  id: CH-agent-comms-loop-contract-canary
  mission: "As Bruno, author and run a fan-out Loop whose run-agent nodes declare output_schema — the adjacent path this cycle explicitly moved code beneath but was never supposed to change — and prove validation, repair and settlement behave exactly as before, with no call record anywhere."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-loop-contract-regime-adoption]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Run a run-agent node three ways against the same declared output_schema: a conforming payload, a violating one, and one over the effective result budget. The conforming payload must be validated both when produced and again when it settles. The violating one must be unable to settle as succeeded and must report the validator's own error text. The oversized one must fail on the budget without leaking its lease."
      - "Prove the two regimes actually converged rather than merely compiling together: feed the same payload and the same contract to a Loop node and to a call's expect, and confirm identical verdicts and identical validator text. Confirm the example-shape shorthand and its expanded schema pin the same digest on both sides."
      - "Confirm the boundary ADR-012 draws: after the run, compozy call list over that workspace shows the Loop created no call records at all. Loops adopt the contract regime, not the call domain."
      - "Walk enough of the surrounding journey to catch collateral damage rather than only the contract seam: author and validate the graph grammar, run a wide fan-out through its bounded active window, let fail_fast or best_effort settle the collect, and confirm run status, completion_state, collect counts and route causes still agree across CLI, HTTP, UDS, native tools and the web."
      - "Interrupt once: restart the daemon mid-window and confirm no lane duplicates, active width stays bounded, and settled run-agent child sessions are actually stopped."
    must_avoid:
      - "Drifting into the call domain's own behaviors — a finding there belongs to its owning charter and to a follow-up, not to this debrief. This session's job is the untouched path."
      - "Substituting a seeded fixture for a real authored run; a canary that does not walk the real path proves nothing."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

The targeted tier's mandatory adjacent canary — one session, on the journey this cycle's diff was not
supposed to reach.

Loops were chosen over Network conversations deliberately. Network looks like the adjacent system
because of the publish bridge, but publish is *our* surface: it is a one-way write we added, it has
its own scenario, and it is walked under `CH-agent-comms-in-session-truth` as targeted work. Loops
are the genuine adjacency, because task_02 moved real code out from under them — action validation,
JSON extraction and repair-prompt rendering left `internal/loop/action_schema.go` for
`internal/contracts` — while ADR-012 promised loops keep their own records and create no call
records. That is a claim of *no behavior change on a path that was rewritten*, which is exactly the
shape of risk a canary exists to catch, and the highest-cost outcome this program could produce is
the contract regime leaking into loop settlement.

`J-complete-partial-loop` is the right anchor rather than a config or catalog journey: it is where
`run-agent` nodes actually produce and settle structured output, and it already proved itself as a
canary anchor in the loop-legibility cycle. Invariant 14 rides along — `internal/contracts` performs
no I/O beyond its registry store, so validation is pure given digest and payload, and the
cross-pipeline equivalence check in the second `must_try` is its observable consequence.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
