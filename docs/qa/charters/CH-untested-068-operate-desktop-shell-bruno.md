# CH-untested-068-operate-desktop-shell-bruno: Settle J-operate-desktop-shell for Bruno

```yaml
charter:
  id: CH-untested-068-operate-desktop-shell-bruno
  mission: "Walk J-operate-desktop-shell as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-web-command-palette-shortcuts, ET-web-desktop-shell-lifecycle, ET-web-dock-default-window-size, ET-web-dock-magnification, ET-web-inter-opsz-medium-510, ET-web-menubar-menu-set, ET-web-sessions-catalog-modal, ET-web-shell-shortcuts-about-dialogs, ET-web-ui-resilience, ET-web-window-routing-lifecycle]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Open desktop apps, sessions, and actions from the keyboard; Operate the desktop shell across workspaces and connection states; Dock apps open at enlarged default window sizes."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
