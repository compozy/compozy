# CH-agent-situation-context: Orient an agent in its authenticated project

```yaml
charter:
  id: CH-agent-situation-context
  mission: "As Ada, read the identity, project, work and capability context supplied to a session, and use it to prepare a project handoff without borrowing another session's authority."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-workspace-context
  scenarios: [RT-031]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create logical sessions in two registered projects through the public CLI and compare their authenticated HTTP and UDS context with independent session/workspace reads."
      - "Inspect section limits and truncation, task availability, immutable session identity, and current native-tool diagnostics; confirm retired product sections are absent."
      - "Omit, mismatch, and stop an identity; require honest refusals with the healthy sibling unchanged, then use a fresh valid identity to recover."
      - "Send one natural project handoff request through an available real provider and retain the delivered kernel-context receipt and the resulting artifact. Provider admission or authentication failure remains a separate unverified leg."
      - "Record whether a public configuration can reach an unconfigured context service; never manufacture a production-parity verdict with a stubbed server."
    must_avoid:
      - "No database reads, source inspection, credential changes, or evaluator framing during the persona session."
      - "No prompts that nudge a stalled provider, unrelated session stops, or synthetic success artifacts."
```

<!-- Immutable charter: record runs in the dated report. -->
