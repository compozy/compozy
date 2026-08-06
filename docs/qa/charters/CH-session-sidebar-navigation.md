# CH-session-sidebar-navigation: Navigate session threads without losing route or focus truth

```yaml
charter:
  id: CH-session-sidebar-navigation
  mission: "As Bruno, switch among nested sessions from an open session window and keep the requested route, visible ancestry, collapse state, and keyboard focus truthful."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-14
  scenarios: [ET-web-session-sidebar-threads]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open the sessions sidebar, filter for a deeply nested session, and confirm its complete ancestor path remains visible."
      - "Switch to a session that already owns another window at a different child route; confirm the existing window lands on the requested route without a duplicate."
      - "Collapse and expand a provenance thread by pointer and keyboard; collapsed child controls must leave the tab order while the root toggle remains reachable."
      - "Reload after changing open and collapse state; confirm the same preferences and current-session marker return."
    must_avoid:
      - "Do not use internal state, source inspection, or browser developer tools to settle the journey."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
