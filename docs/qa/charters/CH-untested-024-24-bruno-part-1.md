# CH-untested-024-24-bruno-part-1: Settle J-24 for Bruno

```yaml
charter:
  id: CH-untested-024-24-bruno-part-1
  mission: "Walk J-24 as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [ET-web-jobs-triggers-catalog, ET-web-tasks-mode-url, TA-003, TA-017, TA-018, TA-019, TA-022, TA-023, TA-027, TA-033]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Jobs and Triggers catalog plus deep-linkable detail; Tasks mode navigation via URL search param; View task detail (tabs)."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
