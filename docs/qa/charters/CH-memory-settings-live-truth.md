# CH-memory-settings-live-truth: Memory settings expose only live controls and preserve daemon truth

```yaml
charter:
  id: CH-memory-settings-live-truth
  mission: "As Dora, inspect and edit Memory settings, prove every visible control maps to daemon behavior, and confirm navigation or reload never resurrects removed or unsaved state."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-runtime-settings
  scenarios: [MS-026]
  tour: Back-Button Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open Settings → Memory from normal navigation; confirm recall signal queue capacity and retry limit remain editable while Signal metrics is absent."
      - "Change one retained signal value, save, refresh, and confirm the same value through a structured settings read."
      - "Navigate away with an unsaved change, use Back, and confirm the last persisted value remains authoritative."
      - "Probe the removed TOML/config/API path through a public structured surface and confirm it is rejected, never silently ignored."
    must_avoid:
      - "Do not infer a verdict from source, mocks, or automated tests."
      - "Do not mutate unrelated settings or use internal storage as the independent read path."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run report. -->
