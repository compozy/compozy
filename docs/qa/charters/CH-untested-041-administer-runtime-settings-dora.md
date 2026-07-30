# CH-untested-041-administer-runtime-settings-dora: Settle J-administer-runtime-settings for Dora

```yaml
charter:
  id: CH-untested-041-administer-runtime-settings-dora
  mission: "Walk J-administer-runtime-settings as Dora and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-runtime-settings
  scenarios: [MS-025, MS-027, MS-036, MS-049, MS-web-entity-modal-shell, MS-web-modal-help-tips, MS-web-settings-takeover-redesign]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: General settings view/edit; Observability settings view/edit; Observability log-tail stream."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
