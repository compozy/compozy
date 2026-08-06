# CH-session-catalog-navigation: Find and open nested sessions from the global catalog

```yaml
charter:
  id: CH-session-catalog-navigation
  mission: "As Bruno, use the global Sessions catalog to find nested work, collapse groups safely, and open the intended session without duplicate windows or hidden focus targets."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-web-sessions-catalog-modal]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open Sessions from the dock and command palette, then filter for a deeply nested session and confirm its complete ancestor path."
      - "Collapse a provenance thread and an agent group by keyboard; hidden row controls must leave the tab order and return after expansion."
      - "Open a filtered session, return to the catalog, dismiss it with Escape, and confirm no duplicate window or unintended selection remains."
      - "Refresh, reopen Sessions, and confirm catalog truth and persisted collapse state."
    must_avoid:
      - "Do not use internal state, source inspection, or browser developer tools to settle the journey."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
