# CH-untested-039-32-ada: Settle J-32 for Ada

```yaml
charter:
  id: CH-untested-039-32-ada
  mission: "Walk J-32 as Ada and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-32
  scenarios: [RT-030, RT-032]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Agent self-identity (`/agent/me`); Bounded agent spawn (`/agent/spawn`)."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
