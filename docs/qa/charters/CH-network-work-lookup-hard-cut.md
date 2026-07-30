# CH-network-work-lookup-hard-cut: Resume one work item through the canonical lookup verb

```yaml
charter:
  id: CH-network-work-lookup-hard-cut
  mission: "As Théo, inspect a Network work item through the public detail route and CLI lookup verb, then prove the removed status verb no longer resolves."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-23
  scenarios: [NB-022]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create or resolve one real directed work item, read it through HTTP/UDS and compozy network work lookup, and compare lineage, state, and timestamps."
      - "Probe an invalid work id and the removed network work status verb; both failures must be deterministic and must not mutate the work item."
    must_avoid:
      - "Expanding into the full Network conversation journey or bridge-provider delivery."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
