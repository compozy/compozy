# CH-untested-037-31-bruno: Settle J-31 for Bruno

```yaml
charter:
  id: CH-untested-037-31-bruno
  mission: "Walk J-31 as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-31
  scenarios: [ET-web-agent-detail-tab-parity, ET-web-agent-fleet-listing-rows, RT-agent-detail-runtime-live-edit, RT-agent-overview-canonical-metrics]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Agent detail tab panelbox and content contracts; Agent fleet listing rows match shared ListingRow grammar; Agent detail live runtime selector mutation."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
