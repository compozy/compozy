# CH-session-empty-and-dock-last-created: Keep the empty tab and open last created

```yaml
charter:
  id: CH-session-empty-and-dock-last-created
  mission: "As Théo then Bruno, prove delete keeps the session window on the empty tab and the Sessions dock opens the last created session unless the catalog is empty."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-014, RT-session-delete-keeps-empty-tab, ET-web-dock-contextual-session-launch]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Delete a focused session and confirm the same window stays on /sessions with the sidebar available."
      - "Confirm HTTP DELETE still removes the catalog row and cannot restore the deleted session through undo."
      - "Click Sessions on an empty catalog to open create, then seed a session without opening a window and click Sessions again."
    must_avoid:
      - "Do not treat an open-window count as catalog emptiness."
      - "Do not use the detached Plus control as the Sessions icon."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
