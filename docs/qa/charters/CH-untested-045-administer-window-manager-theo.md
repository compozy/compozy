# CH-untested-045-administer-window-manager-theo: Settle J-administer-window-manager for Théo

```yaml
charter:
  id: CH-untested-045-administer-window-manager-theo
  mission: "Walk J-administer-window-manager as Théo and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-window-manager
  scenarios: [RT-desktop-pager-overview]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Navigate persistent desktops through the minimal pager."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
