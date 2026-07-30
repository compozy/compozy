# CH-untested-054-connect-bridge-provider-omar: Settle J-connect-bridge-provider for Omar

```yaml
charter:
  id: CH-untested-054-connect-bridge-provider-omar
  mission: "Walk J-connect-bridge-provider as Omar and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Omar
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-connect-bridge-provider
  scenarios: [NB-indeterminate-bridge-delivery]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Handle indeterminate bridge delivery without replay."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
