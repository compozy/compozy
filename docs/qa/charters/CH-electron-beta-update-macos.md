# CH-electron-beta-update-macos: Prove a real beta N to N+1 update on macOS

```yaml
charter:
  id: CH-electron-beta-update-macos
  mission: "As Bruno on macOS, install published beta N and take the real published beta N+1 through the product UI so runtime-first update, installer handoff, restart verification, and recovery hold against signed release assets."
  mode: scenario-based
  platform: macos
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
      - "Publish beta N with the task_04 authority, install its signed/notarized macOS package into an isolated destination, record versions, digests, provenance, operation history, and one active agent run, then publish N+1 only after every immutable asset verifies."
      - "Observe the N+1 menubar indicator, open Settings Updates, apply, defer once, then consent: one O_EXCL operation runs runtime-first, journals before each effect, hands the verified app asset to the installer, and reports N+1 only after post-restart version proof."
      - "Interrupt once while dormant and cancel through a different transport; separately force one post-swap health failure and prove the journal restores the previous runtime before a clean retry converges."
      - "Repeat the read path in Chrome and over HTTP plus UDS; holder, revision, phase, percent, staged/rolled-back truth, and final versions must agree everywhere."
    must_avoid:
      - "Never publish a partial generation, bypass signing/notarization, reuse the operator installation, or describe fixture-feed evidence as the real beta gate."
      - "Do not remove published evidence during the session; any cleanup must follow the release authority's explicit procedure and preserve the audit trail."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
