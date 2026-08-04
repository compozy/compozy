# CH-loop-lifecycle-config-cli: Tune lifecycle policy without losing valid state

```yaml
charter:
  id: CH-loop-lifecycle-config-cli
  mission: "As Ada, tune delivery, watch, and breaker lifecycle policy through structured CLI commands and prove invalid values never damage the last valid configuration."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-tune-loop-lifecycle-defaults
  scenarios: [LP-loop-lifecycle-config-cli]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Set and freshly read representative delivery, watch, and global breaker paths."
      - "Exercise every documented mutable path with a valid value through structured output."
      - "Submit an invalid bound and duration, then confirm the prior valid values remain."
      - "Try the non-mutable autopause path and confirm it is rejected."
    must_avoid:
      - "Reading config files or internal storage as proof instead of a public CLI read."
      - "Parallel config writes against the isolated home."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in the run report. -->
