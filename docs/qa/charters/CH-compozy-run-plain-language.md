# CH-compozy-run-plain-language: A non-technical owner can understand the run truth

```yaml
charter:
  id: CH-compozy-run-plain-language
  mission: "As Cora, open a provided run deep link and decide what ran, which runtime was applied, what finished, and whether anything needs attention without using a terminal or learning internal enums."
  mode: charter-with-tour
  persona:
    name: Cora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-runtime-provenance-observation, LP-loop-run-deep-link]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open the exact /loop-runs/<run_id> URL supplied by the owning runtime session and explain the run's status, applied runtime, output, and next action in plain language."
      - "Reload after daemon restart and confirm the same persisted run appears without a blank, optimistic, or contradictory state."
      - "Check that runtime provenance is inspectable but not editable and that identifiers/enums never replace the primary human explanation."
    must_avoid:
      - "CLI, HTTP, UDS, config editing, or runtime setup — the owning Bruno/Ada charters provide the run."
      - "Judging typography or visual polish beyond whether the shipped truth is understandable."
  coverage:
    tier: targeted-non-technical-lens
    surfaces: [Web-run-detail, deep-link, persisted-runtime-provenance]
    invariants: [5, 13]
    adrs: [ADR-001, ADR-002]
    expected_evidence: "Desktop browser capture plus Cora's plain-language read of status, runtime, output, and next action."
    exit_criteria: "Cora can explain the run correctly without terminal context and sees no unsupported edit control."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

