# CH-electron-shell-safety-macos: Pressure-test the thin Electron shell on macOS

```yaml
charter:
  id: CH-electron-shell-safety-macos
  mission: "As Dora on macOS, interrupt attach, startup, navigation, control, geometry, and quit so the thin Electron shell proves it cannot mutate the runtime, expose a privileged browser surface, or leave a duplicate process."
  mode: strategy-based
  platform: macos
  persona:
    name: Dora
    device: desktop
    network: flaky
    locale: en-US
  journey: J-desktop-attach-daily
  scenarios: [APP-attach-running-daemon, APP-start-installed-daemon, APP-quit-contract, APP-desktop-page-zoom, APP-window-geometry-recovery]
  tour: Interrupt Tour
  time_box_minutes: 90
  cadence_tier: targeted
  hot_spots:
    safety_invariants: [12, 13, 14, 15, 16, 17, 18]
    adrs: [ADR-002, ADR-009]
  guidance:
    must_try:
      - "With one isolated daemon and an active agent run, attach, start from stopped, resize, page-zoom, kill and reopen the app, then quit normally; process and CLI readbacks must prove one daemon and uninterrupted work."
      - "Interrupt bundled bootstrap before its first write and retry; the shell may create ~/.compozy/bin only for an empty first-run home and must render a non-interactive boot state, never an update offer."
      - "Exercise same-origin navigation, a hostile deep link, window.open, permission request/check, and a foreign localhost origin; all off-product rendering is denied while valid https leaves through the OS browser."
      - "Probe the token-authenticated control socket with missing, stale, oversized, parallel, and reused requests; every refusal is bounded and redacted, then a fresh legitimate status/open request succeeds."
    must_avoid:
      - "Do not claim automation on macOS: capture the public app, process table, CLI/API readbacks, response headers, and control transcript as a scripted-manual session."
      - "Do not use the operator home or leave any app, daemon, browser, or watcher alive after the lab teardown."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
