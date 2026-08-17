# CH-electron-install-docs-canary: Follow the public Electron install path as an adjacent canary

```yaml
charter:
  id: CH-electron-install-docs-canary
  mission: "As Cora, follow the public installation and desktop-app docs into real beta packages for macOS and Linux, then use the operator runbook to understand update and recovery without encountering a retired shell path or unsupported promise."
  mode: scenario-based
  platform: macos and linux
  persona:
    name: Cora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [REL-beta-install-paths, REL-beta-installer-provenance, REL-beta-self-update]
  tour: Feature Tour
  time_box_minutes: 60
  cadence_tier: adjacent-canary
  hot_spots:
    safety_invariants: [3, 10, 11, 12]
    adrs: [ADR-002, ADR-003, ADR-009]
  guidance:
    must_try:
      - "Start at README Installation and the rendered site installation and desktop-app pages; download the named macOS and Linux beta artifacts and verify publisher, checksum, architecture, package identity, and bundled runtime."
      - "Follow the release-operator runbook for normal update, staged app, rollback, and repair explanations; every command and status must match the public CLI/API contract."
      - "Search the public docs, official skill, release notes, and current assets for legacy shell markers, the retired external feed host, deleted app-scoped update guidance, old signing keys, and the deleted desktop configuration contract; none may remain as active guidance."
      - "Confirm the Electron cutover note preserves the existing home/package identity and gives portable Linux users the explicit old-AppImage removal step without promising automatic migration."
    must_avoid:
      - "Do not use repository paths as the primary user route or accept generated prose that disagrees with the live CLI, package, or release."
      - "Do not mutate the operator installation; use isolated package destinations and homes on both OSes."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
