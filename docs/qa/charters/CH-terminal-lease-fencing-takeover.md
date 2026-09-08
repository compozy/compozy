# CH-terminal-lease-fencing-takeover: Interrupt the writer at the worst possible moment

> Superseded on 2026-09-04 by `CH-terminal-shared-input-race`. Preserve this file as historical QA
> memory; do not schedule its removed lease/takeover mission.

```yaml
charter:
  id: CH-terminal-lease-fencing-takeover
  mission: "As Marina supervising an agent, interrupt control at every seam the lease model has — mid-write takeover, a second human client, a closed tab, a run that ends, a runtime that restarts — trying to produce two writers, a queued keystroke that lands at a later prompt, or a stale agent whose action still changes something."
  mode: charter-with-tour
  persona:
    name: Marina
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-agent-terminal
  scenarios: [ET-terminal-lease-fencing, ET-terminal-agent-handoff-input]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Take control while the agent is actively typing, repeatedly and at different points in its output; confirm every takeover is whole, that no fragment of agent input reaches the program, and that the agent's next call reports the new controller instead of hanging."
      - "Make the agent write while it does not hold control, then let it take control legitimately; confirm no earlier refused write ever arrives at the new prompt."
      - "Race two human clients for control: decline one confirmation, force the other, and confirm an agent can never take control from a human the way a human takes it from an agent."
      - "End the controlling agent's run, and separately restart the runtime, then let the pre-restart agent act; confirm control returns to the human exactly once and that the stale action leaves no output, record, or state change behind."
      - "Open the same terminal in two tabs as one controller, close one and then the other, and confirm control is held through the first close and released only after the grace period — then confirm a bound agent can claim it afterwards."
      - "Answer an input request as a watcher and as the controller, and take full control while a request is pending; confirm answering is one atomic handoff that returns control to the agent, while a full takeover is what supersedes the request."
    must_avoid:
      - "Do not settle how the typing grant is asked for or revoked — CH-terminal-approval-ladder owns that; do not settle what a redacted answer leaves behind, CH-terminal-redaction-osc-boundary owns that."
```

## Focus areas

- **Safety Invariant 1 (exactly one writer)** and **Safety Invariant 4 (non-controller writes are
  rejected, never queued)** — a queued write reaching a later prompt is the disallowed failure mode.
- **Safety Invariant 2 (generation fencing)** — a stale runtime generation fails with a typed fence and
  zero side effects, and the recovered run must re-establish its view before acting.
- **Safety Invariant 3 (atomic human takeover)** — agent writes in flight behind the transition point
  are rejected outright, never partially applied.
- **Safety Invariant 5 (run end and runtime recovery auto-yield exactly once)** — idempotent and
  event-emitting, so no terminal is ever left locked and no transition is recorded twice.
- **Safety Invariant 16 (answering an input request is one atomic transition)** — the answer path takes
  an ephemeral lease and returns control, and never short-circuits the single-writer rule.
- **ADR-002 (one writer, lease-free watching, human takeover wins)** and **ADR-012 (agent doors and
  generation fencing)** — plus the controller-disconnect grace counted per live controlling attachment,
  so one person with two tabs is one controller.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
