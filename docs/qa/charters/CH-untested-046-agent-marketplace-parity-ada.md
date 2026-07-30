# CH-untested-046-agent-marketplace-parity-ada: Settle J-agent-marketplace-parity for Ada

```yaml
charter:
  id: CH-untested-046-agent-marketplace-parity-ada
  mission: "Walk J-agent-marketplace-parity as Ada and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-031, ET-032, ET-033, ET-035, ET-036, ET-042]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: List resources (filtered); Get resource; Put resource (optimistic CRUD + codec validation)."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
