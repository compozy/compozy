# CH-loop-template-materialization: Resolve Loop inputs once and preserve the authored snapshot

```yaml
charter:
  id: CH-loop-template-materialization
  mission: "As Ada, dry-run and start a templated Loop, then compare its authored and resolved contracts through structured public reads."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-loop-template-snapshot-round-trip]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Dry-run with an input that resolves the Goal narrative and confirm no Run is created."
      - "Start the same Loop and confirm the raw executed definition still carries the authored template while materialized_contract carries the resolved text."
      - "Use literal braces inside an input value and confirm they remain literal instead of becoming a second template."
    must_avoid:
      - "Do not treat source code, generated schemas, or direct database reads as runtime evidence."
      - "Do not reuse the operator's COMPOZY_HOME or provider session."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
