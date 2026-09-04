# CH-provider-switch-settings: Preserve only settings owned by the same provider

```yaml
charter:
  id: CH-provider-switch-settings
  mission: "Verify that structured agent updates preserve provider-owned runtime settings for an equivalent provider identity and clear omitted settings for a real provider change."
  mode: charter-with-tour
  persona: { name: Ada, device: desktop, network: wifi-fast, locale: en-US }
  journey: J-32
  scenarios: [RT-081]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Update an agent from a built-in provider alias to its canonical name without runtime flags"
      - "Update the same agent to a different provider without runtime flags"
      - "Read the agent independently after each update and inspect structured output"
    must_avoid:
      - "Create, duplicate, delete, or start sessions"
      - "Exercise provider credentials or model execution"
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
