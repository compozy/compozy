# CH-untested-valid-022-network-local-default-bruno: Valid companion for J-network-local-default as Bruno

```yaml
charter:
  id: CH-untested-valid-022-network-local-default-bruno
  mission: "Re-run J-network-local-default under the scenario's owning persona and a canonical tour, preserving the historical charter unchanged."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-network-local-default
  scenarios: [NB-participation-controls-serialize, TA-001, TA-004]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Enter through every scenario's named public surface and capture the final observable from a fresh read."
      - "Exercise one failure, interruption, or invalid-input branch without masking it through an alternate surface."
      - "Compare public surfaces wherever parity is part of the expected behavior."
      - "Prioritize these representative observables first: Session/task/automation participation controls serialize Local by default; Create task; Edit task."
    must_avoid:
      - "Do not inherit a verdict from the historical charter or static implementation evidence."
      - "Do not rewrite the historical charter; record this run only in the current report."
```

<!-- Immutable companion charter: historical planning remains untouched. -->
