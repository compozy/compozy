# CH-electron-browser-update-parity: Compare update truth in Chrome and the app

```yaml
charter:
  id: CH-electron-browser-update-parity
  mission: "As Bruno, keep Settings open in Chrome and the Electron app while one update moves through available, blocked, applying, staged, failed, rolled-back, and ready states, proving both doors render the same daemon truth."
  mode: charter-with-tour
  platform: macos and linux
  persona:
    name: Bruno
    device: desktop
    network: flaky
    locale: en-US
  journey: J-desktop-update-moment
  scenarios: [APP-web-update-two-track, APP-web-update-indicator, MS-settings-update-mutations]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  cadence_tier: targeted
  hot_spots:
    safety_invariants: [1, 2, 5, 8, 16]
    adrs: [ADR-006, ADR-009]
  guidance:
    must_try:
      - "Use the manifest-derived proxy target to open the same isolated home in Chrome and the packaged app; compare track presence, versions, action absence/presence, holder, phases, percent, messages, and refresh persistence."
      - "Apply in Chrome while the app is open, attempt a competing app apply, then cancel only after the operation becomes dormant; no surface may claim optimistic success or show a shell-only control."
      - "Exercise the menubar indicator with pointer and keyboard: absent from the DOM outside idle availability, one count-free indicator for either or both tracks, and activation lands on the Updates section."
      - "Attempt an inline script, external frame, and unlisted connect origin through Chrome and the app; both must enforce the exact production CSP."
    must_avoid:
      - "Do not use mocked settings payloads or browser devtools state edits as verdict evidence."
      - "Do not run Chrome and the app against different homes, ports, release generations, or caches."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
