# CH-acp-automatic-runtime-recovery: Keep one long turn alive across a provider disconnect

```yaml
charter:
  id: CH-acp-automatic-runtime-recovery
  mission: "As Théo, keep a long session open through one provider disconnect and confirm the original turn completes without resending or losing its partial answer."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: flaky
    locale: en-US
  journey: J-automatic-runtime-recovery
  scenarios: [RT-acp-automatic-recovery]
  tour: Network Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start one prompt through the public session surface with a provider that emits partial output, disconnects once, then accepts reconstructed context on the replacement process."
      - "Observe the recovery notice without sending another prompt; confirm attempt progress is announced and the existing transcript stays visible."
      - "Read the final transcript, status, and events through fresh web and structured paths; compare the turn id, generation, transition, and terminal markers."
      - "Close the session window during recovery, reopen its permalink, and confirm durable catch-up reaches the same completed turn."
    must_avoid:
      - "Do not resend, steer, or nudge the agent after the disconnect; a silent or stalled recovery is the finding."
      - "Do not inspect SQLite or source code to decide the verdict; use only web, CLI, HTTP, UDS, and native-tool reads."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
