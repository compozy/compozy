# CH-eng-136-dock-contextual-session: Launch or return to sessions from the dock

```yaml
charter:
  id: CH-eng-136-dock-contextual-session
  mission: "As Bruno, use the Sessions dock icon to start a session from a cold desktop, return to the most-recent session window, and reach the catalog through its dedicated control."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-web-dock-contextual-session-launch, ET-web-sessions-catalog-modal]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Click Sessions with no session window and verify the create flow opens directly; choose the available agent, start the session, and verify the new window is visible."
      - "Minimize the created window, click Sessions again, and verify the same window is restored and focused rather than a second session being created."
      - "Use the Session menu or palette Toggle sessions control to open the catalog; select a session and dismiss it without opening the catalog from the dock."
      - "Reload after the focus return and verify the session window and catalog controls still reflect the live shell state."
    must_avoid:
      - "Do not use internal state, source inspection, database queries, or developer tools to settle either scenario."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
