# CH-electron-shell-safety-linux: Pressure-test the thin Electron shell on Linux

```yaml
charter:
  id: CH-electron-shell-safety-linux
  mission: "As Dora on Linux, interrupt attach, startup, navigation, control, geometry, and quit through the packaged Playwright _electron surface so the thin shell proves runtime and browser-boundary safety."
  mode: strategy-based
  platform: linux
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
      - "Drive the packaged `_electron` app against one isolated daemon: attach, stopped start, app/browser coexistence, page zoom, geometry recovery, crash, reopen, and quit with an active agent run."
      - "Interrupt bundled bootstrap before its first write and retry; an empty home may be provisioned once, while an owned install is never rewritten by shell startup."
      - "Assert production BrowserWindow flags and negative behavior through public effects: off-origin navigation, window.open, permissions, remote debugging, iframe embedding, inline script, and an unlisted connect origin all fail closed."
      - "Probe socket ownership, 0700/0600 permissions, rotated token auth, the 64 KiB cap, one request per connection, and bounded concurrency; diagnostics and errors remain redacted."
    must_avoid:
      - "No forced-actionability or devtools bypasses in Playwright; the packaged policy is the product contract."
      - "Do not use the operator home or leave any app, daemon, browser, or watcher alive after teardown."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
