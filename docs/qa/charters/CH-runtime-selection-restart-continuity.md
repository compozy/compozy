# CH-runtime-selection-restart-continuity: Keep one session's runtime choice through interruption

```yaml
charter:
  id: CH-runtime-selection-restart-continuity
  mission: "As Théo, choose a non-default runtime for one session, interrupt the app and daemon, and return to find that exact choice ready for the next prompt."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-session-runtime-selection-continuity]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Choose a provider, model, reasoning level, and speed that differ from the agent default; observe immediate save feedback."
      - "Refresh, stop, reopen, restart the isolated daemon, and revisit the permalink after each interruption."
      - "Send the next prompt and confirm the selected runtime was applied while older turns and the agent default stayed unchanged."
    must_avoid:
      - "Changing the agent's authored default or using an explicit one-prompt override as persistence evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
