# CH-cursor-onboarding-runtime-defaults: Choose truthful Cursor defaults on first run

```yaml
charter:
  id: CH-cursor-onboarding-runtime-defaults
  mission: "As Lea on first run, choose one logical Cursor model with Reasoning and Fast, continue onboarding, and prove the same defaults were saved without exposing a transport alias."
  mode: charter-with-tour
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-19
  scenarios: [RT-071, ET-web-runtime-selector-minimal-slider]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open Cursor in the first-run Runtime selector, choose Grok 4.5 or 4.6, select an advertised reasoning level, toggle Fast, and continue."
      - "Return to the step before completion and confirm the chosen runtime remains; finish setup and read provider settings through the public API or CLI."
      - "Confirm readback contains the logical model, default reasoning, and default_speed=fast, with no private cursor-* alias."
      - "Switch to a model without Fast and confirm the selector normalizes the control instead of preserving an invalid combination."
    must_avoid:
      - "Do not press the catalog Reload control before the first Cursor open; the cold-catalog charter owns that recovery."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
