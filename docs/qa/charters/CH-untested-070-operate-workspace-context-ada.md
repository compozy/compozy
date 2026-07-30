# CH-untested-070-operate-workspace-context-ada: Settle J-operate-workspace-context for Ada

```yaml
charter:
  id: CH-untested-070-operate-workspace-context-ada
  mission: "Walk J-operate-workspace-context as Ada and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-workspace-context
  scenarios: [ET-native-workspace-scope-isolation, ET-web-session-deep-link-isolation, MS-workspace-resolution-chain, MS-workspace-resolution-provenance]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Keep native tool calls inside the caller workspace; Keep session deep links inside the active workspace; Resolve workspace context through one precedence chain."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
