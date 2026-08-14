# CH-dead-session-history-recovery: Return to an interrupted session safely

```yaml
charter:
  id: CH-dead-session-history-recovery
  mission: "As Théo, return to an interrupted session, confirm its history stays readable, and deliberately fork follow-up work without losing the failure evidence."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-dead-session-history-recovery
  scenarios: [RT-acp-stream-disconnect-recovery]
  tour: Back-Button Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open the terminal session through the normal web route and read its partial transcript and failure detail."
      - "Refresh and revisit the original session before choosing recovery."
      - "Use the fork action and verify the child points back to the original while the original stays readable."
      - "Read recap from the CLI or HTTP after the web recovery action to confirm the original evidence remains available."
    must_avoid:
      - "Do not retry the interrupted prompt; its provider-side effects are unknown."
      - "Do not use internal storage or test helpers as user-facing evidence."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
