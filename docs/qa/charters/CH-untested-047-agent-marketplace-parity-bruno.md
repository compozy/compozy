# CH-untested-047-agent-marketplace-parity-bruno: Settle J-agent-marketplace-parity for Bruno

```yaml
charter:
  id: CH-untested-047-agent-marketplace-parity-bruno
  mission: "Walk J-agent-marketplace-parity as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-003, ET-006, ET-009, ET-011, ET-012, ET-038, ET-040, ET-041, ET-043, ET-046]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Skill content viewer (verified body); Enable/disable skill (per-agent tombstone); Skill marketplace install (+visibility verify)."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
