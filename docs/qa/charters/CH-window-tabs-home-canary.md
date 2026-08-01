# CH-window-tabs-home-canary: Confirm session-tab observability did not distort Home

```yaml
charter:
  id: CH-window-tabs-home-canary
  mission: "As Cora, read Home after session-tab activity and confirm usage, cost provenance, attention, and display preferences remain truthful after reload."
  mode: charter-with-tour
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-operate-home-dashboard
  scenarios: [RT-home-usage-window-persistence]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Generate session activity, then compare the Home usage zone with structured observe overview output."
      - "Select 7d, 30d, and 90d, inspect unknown or mixed cost provenance, fold System, and reload."
    must_avoid:
      - "Do not infer cost from tokens when the daemon reports unknown provenance."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
