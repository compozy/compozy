# CH-runtime-ui-regression-update-coherence: Reopen an updated app over its old runtime

```yaml
charter:
  id: CH-runtime-ui-regression-update-coherence
  mission: "As Bruno, launch app N+1 while its app-owned N runtime is healthy and prove the product repairs the version pair without sacrificing home data."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-attach-daily
  scenarios: [APP-desktop-runtime-bundle-coherence]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Record the old runtime PID, version, ownership, home sentinel, and product state before opening app N+1."
      - "Open app N+1, then prove the old owned process stopped, the bundled runtime and provenance agree, one daemon is healthy, and the home sentinel survived."
      - "Interrupt provenance publication once and prove the prior binary stays recoverable without a trusted half-new identity."
    must_avoid:
      - "Do not use the operator home, delete the isolated home, or stop a runtime that is not proven desktop-owned."
      - "Do not substitute the broader consent-driven update flow for this launch-time coherence check."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
