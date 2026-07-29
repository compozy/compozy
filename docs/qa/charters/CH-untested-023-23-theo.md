# CH-untested-023-23-theo: Settle J-23 for Théo

```yaml
charter:
  id: CH-untested-023-23-theo
  mission: "Walk J-23 as Théo and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-23
  scenarios: [NB-006, NB-011, NB-014, NB-016, NB-017, NB-web-network-head-trail]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Create channel (spawns sessions); Post message into a thread (web); Resolve / open direct room."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
