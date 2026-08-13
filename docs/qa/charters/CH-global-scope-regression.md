# CH-global-scope-regression: Keep shell scope truthful while resolution changes

```yaml
charter:
  id: CH-global-scope-regression
  mission: "As Bruno, switch between Global and project scope from the menubar, palette, and a Global-session deep link, and prove pending resolution never performs an action for the wrong scope."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [MS-web-menubar-global-scope-toggle, ET-web-command-palette-shortcuts, MS-web-session-deeplink-global-confirm]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Toggle scope from the globe and command palette, then refresh and confirm the remembered project remains intact."
      - "Open a Global-session deep link while project scope is active; cancel once, then confirm and return to the remembered project."
      - "Attempt the same controls while workspace resolution is visibly pending; no early scope action may run."
    must_avoid:
      - "Do not infer scope state from source, storage inspection, or test fixtures."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
