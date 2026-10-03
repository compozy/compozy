# CH-profile-identity-catalogs: Give the research profile a recognizable identity

```yaml
charter:
  id: CH-profile-identity-catalogs
  mission: "As Dora, create a research context from the dock, choose a precise symbol and custom color, then edit and recognize that identity across the switcher, Settings, and command palette in both themes."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-profiles
  scenarios: [ET-profile-identity-appearance-catalogs]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Search and scroll the complete icon catalog, choose an uncommon icon, create the research profile, and verify the saved identity through HTTP and UDS."
      - "Edit directly from the dock switcher row; search emojis, choose a non-default skin tone, and verify full-width grid layout and local-only emoji data."
      - "Open the spectrum popover without changing dialog height; choose a custom color, check tint and persisted color, and verify the identity across switcher, Settings, and command palette in light and dark themes."
      - "Refuse an invalid icon slug through the CLI, accept a valid one, refresh, and close the dialog with its header X."
    must_avoid:
      - "Do not claim the separate keyboard/screen-reader lifecycle charter or change the operator's original home."
```

The earlier matrix assigned this visual scenario to a CLI-only lifecycle charter. This mission
owns the missing Web catalog and rendering walk; its debrief belongs in the dated report.
