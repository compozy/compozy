# CH-untested-057-evaluate-compozy-beta-dora: Settle J-evaluate-compozy-beta for Dora

```yaml
charter:
  id: CH-untested-057-evaluate-compozy-beta-dora
  mission: "Walk J-evaluate-compozy-beta as Dora and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [REL-beta-install-paths, REL-beta-installer-provenance, REL-beta-self-update]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Install CompozyOS through a documented beta channel; Verify the hosted beta installer through Sigstore; Keep self-update on the running beta line."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
