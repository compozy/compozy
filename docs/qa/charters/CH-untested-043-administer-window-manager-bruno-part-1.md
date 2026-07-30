# CH-untested-043-administer-window-manager-bruno-part-1: Settle J-administer-window-manager for Bruno

```yaml
charter:
  id: CH-untested-043-administer-window-manager-bruno-part-1
  mission: "Walk J-administer-window-manager as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-window-manager
  scenarios: [ET-layout-editor-drag-rebalance, ET-layout-editor-gaps-follow-canvas, ET-layout-editor-group-overlap-refused, ET-layout-editor-load-saved-layout, ET-layout-editor-shortcut-recorder, ET-layout-editor-split-orientation, ET-layout-editor-split-weights, ET-window-manager-drop-swap, ET-window-manager-layout-gestures, ET-window-manager-multi-client]
  tour: Back-Button Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Rebalance a split by dragging its divider in Settings; Editing gaps and snap zones updates the layout canvas at real scale; Dragging a group edge into a neighbour is flagged before it can apply."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
