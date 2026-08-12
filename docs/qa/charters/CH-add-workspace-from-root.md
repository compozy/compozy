# CH-add-workspace-from-root: Reach a project outside home from the picker

```yaml
charter:
  id: CH-add-workspace-from-root
  mission: "As Lea, discover a filesystem root in Add workspace, navigate from it, and register one project only after submit."
  mode: scenario-based
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-add-workspace-by-browsing
  scenarios: [RT-038, MS-051, MS-web-workspace-add-directory-browser]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Compare HTTP and UDS browse roots, then use every displayed Locations action in the real dialog."
      - "Navigate outside home, cancel once to prove no write, then reopen and submit once."
      - "Refresh and confirm the exact chosen root remains registered."
    must_avoid:
      - "Do not type an absolute path as a workaround or inspect the database."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
