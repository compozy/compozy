# CH-electron-offline-first-run-linux: Install the bundled runtime offline on Linux

```yaml
charter:
  id: CH-electron-offline-first-run-linux
  mission: "As Lea on a clean offline Linux home, install the packaged Electron app and reach the product through bundled-runtime provisioning without a terminal, feed request, duplicate daemon, or interactive boot overlay."
  mode: scenario-based
  platform: linux
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
      - "Disconnect before first launch, install the release package, and exercise the packaged Electron app through Playwright `_electron`; pair the harness evidence with a screen recording to prove embedded verify → install → start with zero release-channel request."
      - "Interrupt once before the first install write, relaunch, and retry; corrupt the embedded bundle in a separate package fixture and prove no owned install is created before integrity passes."
      - "Walk boot progress, version skew, and one typed failure; the boot surface stays non-interactive for updates and cannot navigate, open windows, request permissions, or expose devtools."
      - "Quit and relaunch online and offline; the same product opens directly, beta identity stays truthful, and process/CLI evidence shows exactly one daemon."
    must_avoid:
      - "Do not restore network to make provisioning pass or seed the isolated home with an installed runtime."
      - "No dev-mode Electron launch or unpackaged binary counts as release-package evidence."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
