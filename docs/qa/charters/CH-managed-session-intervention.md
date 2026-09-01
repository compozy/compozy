# CH-managed-session-intervention: Correct a managed session without taking over its lifecycle

```yaml
charter:
  id: CH-managed-session-intervention
  mission: "As Théo, open a public daemon-managed session that needs correction, send a prompt when it is stopped, then queue, steer, interrupt, and stop generation while it is active without gaining lifecycle actions."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-13
  scenarios: [RT-018, RT-019]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open a visible system, coordinator, or spawned session from normal in-app navigation and confirm its composer is available."
      - "Send a natural follow-up to an eligible stopped managed session, reload its permalink, and confirm the same session ID keeps the earlier and new transcript."
      - "During a live turn, exercise Queue, Steer, Interrupt, and Stop generation against the visible active turn, then confirm the durable result through a documented structured surface."
      - "Confirm rename, clear, attach, delete, and whole-session stop actions remain unavailable throughout the walk."
    must_avoid:
      - "Dream and hidden maintenance sessions, which are intentionally read-only or absent from the public catalog."
      - "Internal database reads or evaluator language in prompts sent to the agent."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
