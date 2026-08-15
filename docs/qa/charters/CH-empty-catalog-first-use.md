# CH-empty-catalog-first-use: Reach a real next action from every empty catalog

```yaml
charter:
  id: CH-empty-catalog-first-use
  mission: "As Bruno, enter empty Tasks, Jobs, and Triggers catalogs and prove every visible next action is real, scoped, reversible, and truthful after refresh."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-start-from-empty-catalogs
  scenarios: [TA-web-tasks-zero-inventory-templates, TA-web-jobs-zero-inventory-suggestions, TA-web-triggers-zero-inventory-intro]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open all three catalogs from normal navigation in a fresh workspace, then refresh each one."
      - "Expand Task templates and Job suggestions with keyboard and mouse; open the existing Task and Trigger editors, then cancel."
      - "Accept one live Job suggestion and dismiss another, refresh, and confirm both server-owned outcomes."
      - "Force an unmatched Jobs and Triggers search, clear it, and confirm the unfiltered intro returns."
    must_avoid:
      - "Injecting mock suggestions, reading SQLite, or treating Storybook fixtures as live evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
