# CH-untested-069-operate-home-dashboard-cora: Settle J-operate-home-dashboard for Cora

```yaml
charter:
  id: CH-untested-069-operate-home-dashboard-cora
  mission: "Walk J-operate-home-dashboard as Cora and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-operate-home-dashboard
  scenarios: [RT-home-approve-from-dashboard, RT-home-dashboard-zones, RT-home-usage-window-persistence]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Approve and reject task from the home attention zone; Home dashboard renders seven truthful zones; Usage window and system fold persist across reloads."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
