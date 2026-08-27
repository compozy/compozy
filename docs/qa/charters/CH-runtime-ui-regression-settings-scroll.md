# CH-runtime-ui-regression-settings-scroll: Reach the real Settings bottom once

```yaml
charter:
  id: CH-runtime-ui-regression-settings-scroll
  mission: "As Dora, scroll General to its real end and prove the desktop document never becomes a second scroll surface."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-runtime-settings
  scenarios: [MS-settings-single-scroll-owner]
  tour: Back-Button Tour
  time_box_minutes: 15
  guidance:
    must_try:
      - "Measure the document and inner page scroll ranges before and after reaching the bottom."
      - "Send repeated downward wheel input at the bottom and confirm no blank tail appears."
      - "Navigate away and back once to confirm the ownership remains stable."
    must_avoid:
      - "Do not infer the result from CSS source alone."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
