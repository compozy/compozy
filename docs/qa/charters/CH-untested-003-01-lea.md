# CH-untested-003-01-lea: Settle J-01 for Lea

```yaml
charter:
  id: CH-untested-003-01-lea
  mission: "Walk J-01 as Lea and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-action-failure-detail]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Explain a failed Loop action with its preserved cause and recovery path."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
