# CH-native-window-chrome-macos: Use native window chrome without leaving the menubar

```yaml
charter:
  id: CH-native-window-chrome-macos
  mission: "As Dora on macOS, use every native traffic light from the Compozy menubar and compare the browser side by side so window behavior stays native without changing the web shell."
  mode: charter-with-tour
  platform: macos
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-attach-daily
  scenarios: [APP-native-window-controls, APP-window-geometry-recovery, APP-quit-contract]
  tour: Feature Tour
  time_box_minutes: 30
  cadence_tier: targeted
  guidance:
    must_try:
      - "Open the packaged product window and use close, minimize, and the green traffic light from the integrated menubar; the Compozy mark and all menu controls remain unobstructed and clickable."
      - "Move and resize from the draggable menubar and native edges, relaunch after changing window state, and confirm restored geometry remains usable."
      - "Open the same product in a normal browser and confirm the menubar keeps its original leading alignment without desktop controls or a reserved blank strip."
      - "Close the desktop window during active runtime work, then prove through the CLI that the runtime and work continue."
    must_avoid:
      - "Do not use DevTools or DOM-injected window controls; the native title-bar behavior and public browser surface are the contract."
      - "Do not leave the app, daemon, browser, or watcher alive after the lab teardown."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
