# CH-window-manager-review-fixes: Re-walk zoom recovery, stale drops, and the layout summary

```yaml
charter:
  id: CH-window-manager-review-fixes
  mission: "As Bruno, use the desktop and Layouts settings after concurrent changes and confirm every visible zoom, drop, restore, pager, and summary state stays truthful."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-window-manager
  scenarios: [ET-window-zoom-in-place, ET-window-manager-layout-gestures, ET-layout-editor-board-summary, RT-desktop-pager-overview]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Zoom a solo window, open a visible peer, lift over a tiled peer, minimize and restore, then close the zoomed unit while watching the pager and pressed state."
      - "Begin tiled snap and free-drop gestures, advance the layout from another client, and confirm each release rebases from the unit captured at gesture start."
      - "Open Settings Layouts and compare the canvas summary counts and reference label with the active draft."
    must_avoid:
      - "Do not inspect storage or internal state as proof; confirm through Web plus documented CLI or HTTP reads and refresh."
```

<!-- The charter is durable and immutable: debriefs live in dated reports. -->
