# CH-beta-self-update-artifact-contract: Update an isolated beta through the real release archive

```yaml
charter:
  id: CH-beta-self-update-artifact-contract
  mission: "As Dora, update an isolated direct-binary beta through the real signed beta.13 archive and prove the executable reaches the reported version without touching the operator installation."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [REL-beta-self-update]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Build the candidate into the isolated lab as beta.8, record its path and digest, then run `update --check -o json` and `update -o json` from that exact path."
      - "Confirm beta.13 selection, signed artifact verification, executable replacement, final version output, and no mutation of the operator binary."
      - "Exercise abandonment with check-only mode and confirm it leaves the candidate executable unchanged."
    must_avoid:
      - "Do not run the candidate from an operator-managed install path or infer success from unit tests alone."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
