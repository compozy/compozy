# CH-web-exact-model-id: Choose a provider's exact model ID clearly

```yaml
charter:
  id: CH-web-exact-model-id
  mission: "As Bruno, use the runtime selector to enter a real Cursor model ID exactly, persist it, and recover cleanly by keyboard or pointer while the catalog is loading or available."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-web-exact-model-id-entry, RT-session-runtime-selection-continuity]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open Use an exact custom model ID during loading and after the Cursor catalog appears; confirm the labelled field receives focus."
      - "Try empty confirmation, cancel, exact case-sensitive input, Enter, and pointer confirmation with an explicit Cursor target."
      - "Refresh and confirm the exact provider/model through the Web and a public HTTP or UDS readback."
    must_avoid:
      - "Do not submit a provider prompt or spend provider tokens; this journey owns selection and persistence, not agent output."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
