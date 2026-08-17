# CH-native-window-chrome-linux: Follow the Linux desktop's native window layout

```yaml
charter:
  id: CH-native-window-chrome-linux
  mission: "As Dora on Linux, use the desktop environment's native window controls from the Compozy menubar so left, right, or split button layouts never overlap product controls."
  mode: charter-with-tour
  platform: linux
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
      - "Exercise the packaged app under the available desktop environment and record which controls and side its native title-bar setting chooses."
      - "Use every visible native control, drag from empty menubar space, resize from native edges, and confirm every Compozy menu control remains clickable."
      - "Repeat at one narrow usable window width and after maximize/restore so the safe area protects both leading and trailing product controls."
      - "Close during active runtime work and verify the runtime survives through a fresh CLI read."
    must_avoid:
      - "Do not force a three-button macOS-like layout; the Linux desktop environment owns button count and placement."
      - "Do not leave the app, daemon, browser, or watcher alive after the lab teardown."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
