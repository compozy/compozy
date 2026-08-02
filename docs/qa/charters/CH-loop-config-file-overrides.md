# CH-loop-config-file-overrides: Reuse reviewed Loop settings

```yaml
charter:
  id: CH-loop-config-file-overrides
  mission: "As Bruno, preview and persist nested Loop settings from JSON and YAML files, then confirm invalid input leaves the saved configuration intact."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-configure-and-run-loop
  scenarios: [LP-loop-config-file-snake-case]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Use both JSON and YAML through loop run --config-file and loop configure --file."
      - "Include nested enabled_checks_json values and confirm a fresh structured read preserves them."
      - "Add one unknown field and confirm the last valid configuration remains unchanged."
    must_avoid:
      - "Reading configuration storage directly or treating parser tests as the public verdict."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
