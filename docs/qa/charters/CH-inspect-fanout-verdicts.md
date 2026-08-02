# CH-inspect-fanout-verdicts: Inspect fan-out verdict identity

```yaml
charter:
  id: CH-inspect-fanout-verdicts
  mission: "As Bruno, inspect one Loop generation with sibling gate verdicts and prove every public detail surface preserves gate_id plus item_index."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-improve-loop-with-feedback
  scenarios: [LP-fanout-verdict-identity]
  tour: Feature Tour
  time_box_minutes: 20
  guidance:
    must_try:
      - "Read one generation containing two verdicts with the same gate_id and different item_index values."
      - "Compare HTTP, UDS or CLI structured output, and the generated public schema."
    must_avoid:
      - "Reading SQLite directly or treating only a generated type as runtime proof."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
