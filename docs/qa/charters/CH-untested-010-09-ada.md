# CH-untested-010-09-ada: Settle J-09 for Ada

```yaml
charter:
  id: CH-untested-010-09-ada
  mission: "Walk J-09 as Ada and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-09
  scenarios: [TA-063, TA-064, TA-066, TA-073, TA-074]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Loop-target automation job start; Loop-target trigger and webhook start; Loop start binding parity."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
