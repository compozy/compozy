# CH-create-destination-regression: Keep creation state inside its destination

```yaml
charter:
  id: CH-create-destination-regression
  mission: "As Dora, move creation dialogs between project and Global destinations and prove drafts, credentials, selections, and submitted resources stay inside the destination named by the menubar."
  mode: strategy-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-31
  scenarios: [MS-web-create-destination-derived]
  tour: Back-Button Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open Knowledge, task, session, and MCP creation from project scope, abandon, switch Global, and reopen."
      - "Create one Knowledge item and confirm the visible list switches to and selects its destination."
      - "Refresh after creation and confirm the destination and resource remain truthful."
    must_avoid:
      - "Do not use internal cache or store inspection as confirmation."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
