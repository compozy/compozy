# CH-untested-067-operate-daemon-schema-bruno: Settle J-operate-daemon-schema for Bruno

```yaml
charter:
  id: CH-untested-067-operate-daemon-schema-bruno
  mission: "Walk J-operate-daemon-schema as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-daemon-schema
  scenarios: [RT-preserve-corrupt-database-family, RT-refuse-cross-stream-legacy-marker, RT-refuse-legacy-session-database]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Preserve a corrupt database family; Refuse either legacy marker in the shared database; Refuse incompatible session event databases on reads."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
