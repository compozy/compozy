# CH-provider-model-default-speed: Curate and resolve a model speed default

```yaml
charter:
  id: CH-provider-model-default-speed
  mission: "As Ada, persist a provider model's default speed through every structured surface and prove runtime resolution uses it below an authored agent speed."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-20
  scenarios: [MS-054]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Curate a Cursor model with default_effort and default_speed=fast through CLI, HTTP/UDS, and the native descriptor."
      - "Fresh-read provider settings and config through public surfaces; compare the same logical model defaults and live apply generation."
      - "Resolve an agent that omits speed and one that explicitly selects normal; confirm the model default applies only to the first."
      - "Request Fast for a model that explicitly advertises only normal and confirm speed_rejected with no partial write."
    must_avoid:
      - "Do not inspect SQLite or private Cursor transport aliases as verification."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
