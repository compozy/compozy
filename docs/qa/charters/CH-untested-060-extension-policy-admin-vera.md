# CH-untested-060-extension-policy-admin-vera: Settle J-extension-policy-admin for Vera

```yaml
charter:
  id: CH-untested-060-extension-policy-admin-vera
  mission: "Walk J-extension-policy-admin as Vera and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Vera
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-policy-admin
  scenarios: [ET-013, ET-015, ET-050]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Skill settings: registry policy; List extensions; Set extension trust root."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
