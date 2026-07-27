# CH-compozy-beta-candidate: Approve the local candidate without publishing it

```yaml
charter:
  id: CH-compozy-beta-candidate
  mission: "As Dora, produce finite pre-publish proof for the pinned release planner, normalized migration guides, complete legacy disposition, and one truthful local beta-channel contract while changing no external state."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-approve-compozy-beta-candidate
  scenarios: [REL-release-candidate-plan, REL-migration-guide-parity, REL-beta-channel-contract]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Resolve both github.com/compozy/releasepr@v0.0.24 pins; run one read-only beta plan against the explicit candidate and prove release_commit equals checked-out HEAD."
      - "Exercise leading-v, local tag, and origin tag collision rejection; record all nine authoritative outputs and trace them through downstream workflow inputs without re-derivation."
      - "Confirm the workflow, not the planner, retains annotated tag creation, then stop before tag creation, push, or publication."
      - "Run make migration-guide-check, compare all eight sections, audit every legacy CLI/Web/extension/SDK disposition, and inspect README/site/skill/installer/update/package beta truth locally."
    must_avoid:
      - "Creating or pushing tags, releases, packages, checksums, signatures, installer artifacts, redirects, or DNS changes."
      - "Calling live npm, Go proxy, GitHub release, installer, registry, or cosign acceptance as if the beta already existed."
      - "Executing the Task-14 config migrator or first-boot legacy-state probe."
  coverage:
    tier: targeted
    surfaces: [release-workflow, releasepr-skill, git-ref-policy, migration-guides, disposition-ledger, README, local-site, official-skill, installer-source, update-contract, package-metadata]
    invariants: [10, 15]
    deferred_invariants: [11]
    adrs: [ADR-005, ADR-006]
    expected_evidence: "Pin resolution, ref/tag guard transcripts, nine-output trace, no-rederivation map, guide parity output, ledger audit, and local beta-copy captures."
    exit_criteria: "The exact candidate is locally approved with zero external side effects; Task 10 receives the later publish/post-publish handoff."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

