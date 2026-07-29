# CH-untested-020-22-dora: Settle J-22 for Dora

```yaml
charter:
  id: CH-untested-020-22-dora
  mission: "Walk J-22 as Dora and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-22
  scenarios: [MS-provider-detail-modal, MS-web-provider-auth-gate, MS-web-settings-providers-redesign, RT-026]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Provider detail opens as a centered modal with overlay dismissal; Provider editor offers credential controls only under bound-secret ownership; Providers page toolbar, status copy, and Rows/Cards views."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
