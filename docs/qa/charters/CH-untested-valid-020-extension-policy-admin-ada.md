# CH-untested-valid-020-extension-policy-admin-ada: Valid companion for J-extension-policy-admin as Ada

```yaml
charter:
  id: CH-untested-valid-020-extension-policy-admin-ada
  mission: "Re-run J-extension-policy-admin under the scenario's owning persona and a canonical tour, preserving the historical charter unchanged."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-policy-admin
  scenarios: [ET-017, ET-018, ET-ext-curated-digest-verify]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Enter through every scenario's named public surface and capture the final observable from a fresh read."
      - "Exercise one failure, interruption, or invalid-input branch without masking it through an alternate surface."
      - "Compare public surfaces wherever parity is part of the expected behavior."
      - "Prioritize these representative observables first: Install extension (local path + checksum); Install extension (marketplace slug + trust); Verify a curated extension archive against the feed digest."
    must_avoid:
      - "Do not inherit a verdict from the historical charter or static implementation evidence."
      - "Do not rewrite the historical charter; record this run only in the current report."
```

<!-- Immutable companion charter: historical planning remains untouched. -->
