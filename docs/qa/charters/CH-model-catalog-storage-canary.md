# CH-model-catalog-storage-canary: Keep model catalog refresh state truthful after restart

```yaml
charter:
  id: CH-model-catalog-storage-canary
  mission: "As Ada, use the model-catalog refresh and status spine as an adjacent persistence canary, proving optional source timestamps and failure detail survive the redesigned store without leaking secrets."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-20
  scenarios: [MS-043, MS-044]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Run `agh provider models refresh -o json` and `agh provider models status -o json` against the same isolated home; accept empty optional timestamps as empty values, never SQL/nullability failures."
      - "Restart the daemon and read status again; persisted source state must remain parseable and associated with the correct provider/source."
      - "If a deterministic failing source fixture is available, confirm the error retains source context while redacting credentials and prior catalog rows remain stale-marked rather than disappearing."
    must_avoid:
      - "Full model curation, session reasoning, web selector, or external-provider authentication; CH-031 and related charters own those journeys."
      - "Treating an unavailable optional failing-source fixture as failure; record that branch as skipped in the execution report."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
