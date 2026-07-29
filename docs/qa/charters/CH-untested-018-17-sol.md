# CH-untested-018-17-sol: Settle J-17 for Sol

```yaml
charter:
  id: CH-untested-018-17-sol
  mission: "Walk J-17 as Sol and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [ET-web-runtime-selector-minimal-slider]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Minimal runtime selector trigger and reasoning slider."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
