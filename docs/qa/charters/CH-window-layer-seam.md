# CH-window-layer-seam: Keep floating windows above structural seams

```yaml
charter:
  id: CH-window-layer-seam
  mission: "As Bruno, overlap a floating window with a tiled resize seam and keep both the covered window and uncovered seam operable."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-window-manager
  scenarios: [ET-window-manager-layout-gestures]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Tile two windows, place a floating window across their seam, and interact at the overlap point."
      - "Drag an uncovered segment of the same seam and prove both tiled siblings resize."
      - "Refresh and confirm the topology and weights remain durable."
    must_avoid:
      - "Do not change semantic focus ordering or global z-index tokens to force the outcome."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
