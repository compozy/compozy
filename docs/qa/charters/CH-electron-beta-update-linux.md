# CH-electron-beta-update-linux: Prove a real beta N to N+1 update on Linux

```yaml
charter:
  id: CH-electron-beta-update-linux
  mission: "As Bruno on Linux, install published beta N and take the real published beta N+1 through the packaged Electron and browser surfaces so runtime-first update, AppImage staging, restart verification, and rollback hold against signed release assets."
  mode: scenario-based
  platform: linux
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-update-moment
  scenarios: [APP-app-auto-update, APP-runtime-update-app-owned, APP-runtime-update-managed, APP-update-recovery-state, APP-web-update-two-track, APP-web-update-indicator, APP-brand-channel-visibility, APP-abandoned-install-update-polling, MS-settings-update-mutations]
  tour: Interrupt Tour
  time_box_minutes: 90
  cadence_tier: targeted
  hot_spots:
    safety_invariants: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 16]
    adrs: [ADR-002, ADR-009]
  guidance:
    must_try:
      - "Install the real beta N AppImage and bundled runtime in an isolated Linux home, record versions/digests/provenance, start agent work, then consume task_04's verified N+1 publication."
      - "From Settings, defer once and apply once: the durable record is acquired atomically, runtime phases finish first, a running AppImage applies only its recorded digest, and a closed app remains staged until next launch."
      - "Corrupt one candidate before acquisition and prove verification fails before quiesce or install movement; force one post-swap health failure and prove restore plus rolled-back truth before retry."
      - "Compare packaged `_electron`, plain Chrome, CLI, HTTP, and UDS throughout; the production CSP must block inline script, framing, and an unlisted connect origin on both UI doors."
    must_avoid:
      - "A deb package is recommendation-only for the app track; never reinterpret it as Linux self-apply evidence."
      - "Production credentials stay inside the release authority; never copy them into a QA home, operator home, or package."
      - "The real gate accepts only isolated evidence from signed published packages; unsigned local packages cannot establish the verdict."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
