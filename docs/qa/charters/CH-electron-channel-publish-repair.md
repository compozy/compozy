# CH-electron-channel-publish-repair: Publish and repair one complete Electron beta generation

```yaml
charter:
  id: CH-electron-channel-publish-repair
  mission: "As Dora, publish one complete Electron beta generation and repair it to a prior known-good generation, proving immutable assets lead the ref-CAS channel flip and no partial platform state becomes visible."
  mode: strategy-based
  platform: GitHub release authority
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-publish-compozy-beta
  scenarios: [REL-beta-channel-contract, REL-channel-repair-known-good, REL-electron-cutover-announcement, REL-release-archive-update-contract]
  tour: Garbage Tour
  time_box_minutes: 90
  cadence_tier: targeted
  hot_spots:
    safety_invariants: [3, 10, 11]
    adrs: [ADR-003, ADR-007, ADR-008, ADR-009]
  guidance:
    must_try:
      - "Run the task_04 dry-run without secrets and prove fail-closed, then publish only after every macOS/Linux payload, checksum, signature, compat catalog, and platform manifest verifies through an independent public read."
      - "Race a stale channel ref and omit one referenced rollback asset in separate rehearsals; both operations must refuse before the ref moves or any partial generation is visible."
      - "Run the release archive compatibility hook against one accepted and one rejected archive; record compressed size, extracted executable size, and the exact policy decision before publication."
      - "Repair both platform manifests to one known-good generation with a single operation id and audit commit, then retry that id and prove idempotent convergence rather than a second flip."
      - "Compare public release notes and installation/desktop/runbook docs with exact asset names, beta-only policy, manual cutover guidance, and the absence of the retired feed/domain."
    must_avoid:
      - "Never move `channel-beta` before immutable verification, publish `desktop/stable/`, or leave a draft/failed release looking complete."
      - "Do not repair only one platform or describe local workflow inspection as proof of public channel state."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
