# CH-window-tabs-supervisor-recovery: Recover a multi-agent desktop after interruption

```yaml
charter:
  id: CH-window-tabs-supervisor-recovery
  mission: "As Théo supervising several agent sessions, interrupt a tabbed desktop, close work at multiple scopes, and recover the exact session context after reload."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-organize-tabbed-work
  scenarios: [ET-window-tab-close-reopen, ET-window-tab-pinning, ET-window-tab-navigation-stack, ET-window-manager-multi-client, ET-web-desktop-shell-lifecycle, RT-desktop-pager-overview]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Pin one session, close a tab and a group, then reload before reopening both entries newest-first."
      - "Leave one client on another desktop while a peer selects a different active tab in the same shared frame."
      - "Navigate a session tab before interruption and confirm route depth, attention, and active member after restore."
      - "Use the pager as the adjacent canary and confirm cross-desktop activation switches only the current client."
    must_avoid:
      - "Do not use implementation state or browser storage as the deciding read path; confirm through rendered state and daemon output."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
