# CH-implement-tasks-import-parity: Match the extension parser to Loop consumption

```yaml
charter:
  id: CH-implement-tasks-import-parity
  mission: "As Ada, call ext__dev_cycle__import_tasks through a public structured surface and prove implement-tasks consumes the same task graph."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-07
  scenarios: [LP-045]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Invoke the extension tool with the same workspace-relative pattern authored in implement-tasks."
      - "Compare the returned task graph with the load_tasks output visible through the Loop run."
      - "Confirm malformed and empty inputs return deterministic structured errors."
    must_avoid:
      - "Review-and-fix behavior or provider-specific task implementation output."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
