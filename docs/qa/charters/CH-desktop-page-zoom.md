# CH-desktop-page-zoom: Scale the desktop product with platform shortcuts

```yaml
charter:
  id: CH-desktop-page-zoom
  mission: "As Dora on macOS, scale and reset the daemon-served desktop product with standard page-zoom shortcuts without invoking Compozy window Zoom."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-attach-daily
  scenarios: [APP-desktop-page-zoom]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Use Command-plus, Command-minus, and Command-zero in the native main window and observe whole-product scale."
      - "Use the in-product window Zoom control separately and confirm it still changes one Compozy window only."
      - "Relaunch and confirm the app remains attached to the same healthy runtime."
    must_avoid:
      - "Do not treat browser-only zoom or a static capability manifest as the native observable."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
