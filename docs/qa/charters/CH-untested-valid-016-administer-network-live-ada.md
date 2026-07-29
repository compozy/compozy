# CH-untested-valid-016-administer-network-live-ada: Valid companion for J-administer-network-live as Ada

```yaml
charter:
  id: CH-untested-valid-016-administer-network-live-ada
  mission: "Re-run J-administer-network-live under the scenario's owning persona and a canonical tour, preserving the historical charter unchanged."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-network-live
  scenarios: [ET-030, NB-001, NB-023]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Enter through every scenario's named public surface and capture the final observable from a fresh read."
      - "Exercise one failure, interruption, or invalid-input branch without masking it through an alternate surface."
      - "Compare public surfaces wherever parity is part of the expected behavior."
      - "Prioritize these representative observables first: Bundle network settings; Network runtime status; Bundle network onboarding settings."
    must_avoid:
      - "Do not inherit a verdict from the historical charter or static implementation evidence."
      - "Do not rewrite the historical charter; record this run only in the current report."
```

<!-- Immutable companion charter: historical planning remains untouched. -->
