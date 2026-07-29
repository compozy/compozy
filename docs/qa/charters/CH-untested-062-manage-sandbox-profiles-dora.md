# CH-untested-062-manage-sandbox-profiles-dora: Settle J-manage-sandbox-profiles for Dora

```yaml
charter:
  id: CH-untested-062-manage-sandbox-profiles-dora
  mission: "Walk J-manage-sandbox-profiles as Dora and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-manage-sandbox-profiles
  scenarios: [MS-030, MS-web-sandbox-profile-advanced, RT-037]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Sandboxes settings CRUD; Sandbox profile editor exposes lifecycle, network, and Daytona behind Advanced; Sandbox profile management."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
