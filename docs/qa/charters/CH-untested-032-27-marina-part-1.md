# CH-untested-032-27-marina-part-1: Settle J-27 for Marina

```yaml
charter:
  id: CH-untested-032-27-marina-part-1
  mission: "Walk J-27 as Marina and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Marina
    device: phone-large
    network: 4g
    locale: en-US
  journey: J-27
  scenarios: [TA-087, TA-088, TA-089, TA-090, TA-091, TA-099, TA-100, TA-101, TA-103, TA-105]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Goal session snapshot; Externally started Goal discovery; Goal prompt-kind transcript metadata."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
