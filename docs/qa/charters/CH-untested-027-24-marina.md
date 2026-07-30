# CH-untested-027-24-marina: Settle J-24 for Marina

```yaml
charter:
  id: CH-untested-027-24-marina
  mission: "Walk J-24 as Marina and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Marina
    device: phone-large
    network: 4g
    locale: en-US
  journey: J-24
  scenarios: [TA-039]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Task dashboard."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
