# CH-compozy-mixed-runtime-delivery: A mixed run reports the runtimes it actually used

```yaml
charter:
  id: CH-compozy-mixed-runtime-delivery
  mission: "As Bruno, launch one mixed software-delivery run and prove runtime selection, persisted applied provenance, restart truth, workspace containment, and the created-run deep link agree across every management surface."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-runtime-selection-overrides, LP-runtime-provenance-observation, LP-loop-run-deep-link]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Submit ordered id, type, and complexity runtime rules plus repeatable slash-safe --runtime overrides; compare dry-run resolution over CLI, HTTP, and UDS."
      - "Run a seeded mixed task batch and compare the applied provider/model/reasoning and per-field source in CLI status, HTTP, UDS, compozy__loop_status, SSE, and Web Inspect."
      - "Restart the daemon, reopen the printed effective-port /loop-runs/<run_id> URL, and prove durable status plus runtime provenance are unchanged."
      - "Try the same run id from a second workspace and confirm list/read/stream/invoke denial; confirm the Web exposes no runtime edit control."
    must_avoid:
      - "Using a parent loop runtime as evidence for child task binding."
      - "Accepting raw resolver intent when the daemon-applied binding disagrees."
  coverage:
    tier: targeted
    surfaces: [CLI, HTTP, UDS, native-tools, daemon-binder, SQLite, SSE, Web-run-inspect]
    invariants: [1, 2, 3, 4, 5, 13]
    adrs: [ADR-001, ADR-002]
    expected_evidence: "Ordered resolver output, session bindings, persisted status parity, deep-link capture, restart proof, and workspace denials."
    exit_criteria: "Every task reports the runtime it actually received, every read surface agrees after restart, and dry-run never emits a deep link."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

