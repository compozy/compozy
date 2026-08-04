# CH-stopped-session-prompt-continuity: Continue one stopped session without losing its conversation

```yaml
charter:
  id: CH-stopped-session-prompt-continuity
  mission: "As Théo, stop a real session, leave and return, then send a normal follow-up prompt and prove the same durable conversation continues without a separate resume step."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-13
  scenarios: [RT-018]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start a real provider turn, stop the session, and confirm the stopped thread keeps its transcript and composer available."
      - "Close and reopen the session permalink, send a natural follow-up, and confirm one session id contains the earlier and new turns after refresh."
      - "Compare the fresh Web state with session status and transcript through a documented structured surface."
    must_avoid:
      - "Attach/resume lease behavior and busy-input mutation; those are separate contracts."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
