# CH-daemon-schema-parity: Prove fresh daemon schema parity across structured surfaces

```yaml
charter:
  id: CH-daemon-schema-parity
  mission: "As Ada, start Compozy from a fresh home and prove the global schema stream is the only stream (no memory entry) while HTTP, UDS, and CLI report one identical structured state."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-daemon-schema
  scenarios: [RT-inspect-schema-streams, RT-preserve-shared-schema-isolation, RT-001]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start `compozy daemon start --foreground` with a fresh isolated COMPOZY_HOME; capture readiness plus the `store.migrations.applied` event for the global stream."
      - "Read `GET /api/status` over HTTP and over the daemon UDS, then run `compozy status -o json`; compare the `schema_streams` arrays field-for-field and in order."
      - "Require exactly one global entry with a non-empty sum_digest and a version/applied_count that match the shipped migration head, and no memory entry; confirm the broader status envelope remains redacted and usable."
      - "Run `compozy workspace list -o json`, restart the daemon, and repeat status plus the domain read to smoke persistence."
    must_avoid:
      - "Web or Playwright checks; the additive field is intentionally unrendered."
      - "Direct SQLite inspection as the verdict source; table-level behavior belongs to automated store tests."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
