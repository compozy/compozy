# CH-daemon-schema-parity: Prove fresh daemon schema parity across structured surfaces

```yaml
charter:
  id: CH-daemon-schema-parity
  mission: "As Ada, start AGH from a fresh home and prove the global and memory schema streams remain independent while HTTP, UDS, and CLI report one identical structured state."
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
      - "Start `agh daemon start --foreground` with a fresh isolated AGH_HOME; capture readiness plus `store.migrations.applied` events for global and memory."
      - "Read `GET /api/status` over HTTP and over the daemon UDS, then run `agh status -o json`; compare the `schema_streams` arrays field-for-field and in order."
      - "Require exactly global and memory entries with version 1, applied_count 1, and non-empty sum_digest values; confirm the broader status envelope remains redacted and usable."
      - "Run `agh workspace list -o json` and `agh memory list -o json`, restart the daemon, and repeat status plus domain reads to smoke shared-file isolation and persistence."
    must_avoid:
      - "Web or Playwright checks; the additive field is intentionally unrendered."
      - "Direct SQLite inspection as the verdict source; table-level isolation belongs to automated store tests."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
