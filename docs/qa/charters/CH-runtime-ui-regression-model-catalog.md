# CH-runtime-ui-regression-model-catalog: Open a cold model catalog promptly

```yaml
charter:
  id: CH-runtime-ui-regression-model-catalog
  mission: "As Sol, open the first runtime selector after startup and use persisted models while live providers refresh in the background."
  mode: scenario-based
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-model-catalog-cold-open]
  tour: Feature Tour
  time_box_minutes: 15
  guidance:
    must_try:
      - "Measure from the first click until model rows are usable."
      - "Confirm the rows contain persisted provider/model data rather than placeholders."
      - "Inspect runtime errors and stop the daemon while refresh ownership is active."
    must_avoid:
      - "Do not pre-open the selector or count a warmed second open as cold evidence."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
