# CH-window-tabs-keyboard-flow: Organize parallel work without leaving the keyboard

```yaml
charter:
  id: CH-window-tabs-keyboard-flow
  mission: "As Bruno, organize several app and session instances into persistent tab frames and confirm every keyboard and palette action preserves the intended work."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-organize-tabbed-work
  scenarios: [ET-window-tab-deck-lifecycle, ET-window-tab-multi-instance, ET-window-tab-palette-search, ET-window-tab-navigation-stack, ET-window-tab-strip-relocation, ET-web-window-routing-lifecycle, ET-web-dock-default-window-size]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Group through drag, a launch-surface menu, and Command-T; cancel one destination picker."
      - "Cycle two instances from the dock, then find both through Command-K after minimizing one."
      - "Push and pop a drill-in route in one tab, reload, and confirm its sibling kept an independent URL."
      - "Confirm Tasks and Marketplace peer navigation moved to the toolbar strip without a topbar nav zone."
    must_avoid:
      - "Do not repair topology through raw layout mutation; this charter measures ordinary operator paths."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
