# CH-hidden-window-live-resources: Hidden apps release browser work and catch up cleanly

```yaml
charter:
  id: CH-hidden-window-live-resources
  mission: "As Bruno, keep two live OS apps side by side, hide and restore each through every shell path, and prove only user-visible windows own browser streams, polling, and clocks while restored apps catch up without reload."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-hidden-window-live-resources, RT-visible-session-streaming]
  tour: Multi-Tab Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Keep two live windows visible and confirm both continue updating while focus moves between them."
      - "Minimize, switch desktops, activate another stack tab, and background the document while observing stream/poll ownership in the browser Network panel."
      - "Restore each window and confirm current server state appears without reload or duplicate events."
      - "Press a pending session decision shortcut while its window is hidden and confirm only the visible request can act."
    must_avoid:
      - "Do not stop daemon-side sessions or tasks; only browser resource ownership is under test."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
