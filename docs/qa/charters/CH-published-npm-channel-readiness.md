# CH-published-npm-channel-readiness: Close a beta only after npm channels converge

```yaml
charter:
  id: CH-published-npm-channel-readiness
  mission: "As Dora, follow one production beta through both npm publications and the bounded channel-readiness check, proving propagation delay is tolerated without hiding terminal policy failures."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-publish-compozy-beta
  scenarios: [REL-published-npm-channel-readiness]
  tour: Network Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Capture the exact publish completion times, every readiness attempt, and the final public dist-tags for both packages."
      - "Confirm stale or absent tags are retried only within the declared deadline and that latest remains unchanged during beta publication."
      - "Preserve the last registry observation when the deadline expires or a terminal query or policy error stops the run."
    must_avoid:
      - "Republishing an immutable npm version, moving a release tag, or treating an old dist-tag as proof that npm publish failed."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
