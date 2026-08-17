# CH-electron-offline-first-run-macos: Install the bundled runtime offline on macOS

```yaml
charter:
  id: CH-electron-offline-first-run-macos
  mission: "As Lea on a clean offline macOS home, install the signed Electron package and reach the product through bundled-runtime provisioning without a terminal, feed request, duplicate daemon, or interactive boot overlay."
  mode: scenario-based
  platform: macos
  persona:
    name: Lea
    device: laptop
    network: flaky
    locale: en-US
  journey: J-desktop-first-run
  scenarios: [APP-install-first-run-provision]
  tour: Network Tour
  time_box_minutes: 60
  cadence_tier: targeted
  hot_spots:
    safety_invariants: [3, 12, 13, 14, 15, 16]
    adrs: [ADR-002, ADR-006]
  guidance:
    must_try:
      - "Disconnect before first launch, install the signed/notarized beta package, and record verify → install → start progress for the embedded lockstep runtime with zero release-feed request."
      - "Interrupt once before the first install write, relaunch, and retry; an invalid embedded digest must fail before `$COMPOZY_HOME/bin` changes, while a valid bundle converges to one healthy daemon."
      - "Keep the boot window non-interactive through startup, version skew, and one typed failure; it may offer retry/diagnostics where contracted but never an update decision."
      - "Quit and relaunch online and offline; both land directly in the same product, preserve beta branding, and do not re-provision or spawn a second daemon."
    must_avoid:
      - "Do not restore network to make provisioning pass or use a pre-existing runtime/home."
      - "No unsigned package or source-tree launch counts as Gatekeeper or bundled-runtime evidence."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
