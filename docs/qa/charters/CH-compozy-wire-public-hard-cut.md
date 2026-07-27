# CH-compozy-wire-public-hard-cut: Wire, secret, and public identity agree on Compozy

```yaml
charter:
  id: CH-compozy-wire-public-hard-cut
  mission: "As Dora, prove the candidate persists and presents only the Compozy wire and public identity while mixed-case claim secrets remain inside the lease boundary."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-validate-compozy-hard-cut
  scenarios: [NB-compozy-wire-identity, RT-compozy-claim-token-redaction, ET-compozy-public-brand-navigation]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Inspect compozy-network/v0, compozy.runtime, all eight compozy.* extension keys, direct-room derivation, Loop DSL, and protocol docs across CLI, HTTP, UDS, native tools, events, and local Web output."
      - "Plant lowercase and uppercase-shaped compozy_claim_ fixtures; grep logs, diagnostics, SSE, events, transcripts, Web responses, and persisted payloads while confirming hashes and correlation ids survive."
      - "Build the site locally and inspect root/launch routes, metadata, OpenGraph, sitemap, robots, RSS, llms output, generated guidance, and the sole permanent old-slug redirect."
      - "Attempt cross-workspace network and claim reads; the second workspace must not list, read, stream, or mutate the first workspace's data."
    must_avoid:
      - "Live DNS or hosting changes."
      - "Treating truthful historical route input as an active legacy product identity."
  coverage:
    tier: targeted
    surfaces: [network-CLI, HTTP, UDS, native-tools, events, redaction, SQLite, Web, site, generated-docs]
    invariants: [7, 10, 13]
    adrs: [ADR-005, ADR-006]
    expected_evidence: "Wire payloads, zero-hit secret sweeps, workspace-denial results, and local route/metadata captures."
    exit_criteria: "No retired wire identifier or raw claim secret appears, and every local public surface names the canonical Compozy origin."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

