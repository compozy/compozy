# CH-window-tabs-agent-parity: Manage tab topology without the web UI

```yaml
charter:
  id: CH-window-tabs-agent-parity
  mission: "As Ada, drive the complete tab lifecycle through structured surfaces and stop on any parity, isolation, hook, config, or deterministic-error mismatch."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-manage-window-tabs
  scenarios: [ET-window-tab-agent-parity, ET-window-tab-v3-discard, ET-window-manager-public-parity, ET-window-manager-hooks-resources, ET-window-manager-layout-recovery, MS-configure-window-manager, MS-layout-profile-cli-roundtrip]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Compare the same grouped topology through CLI JSON, HTTP, UDS, native tool descriptors, and layout watch."
      - "Trigger not-stacked, pinned-close, and stale-revision errors and prove topology, history, client state, and hooks did not advance."
      - "Set nav_stack_limit and closed_entry_limit sequentially, then confirm the next relevant mutation enforces each write-time cap."
      - "Round-trip v3, reject v2, and verify the official skill plus configuration pages name the live contract exactly."
    must_avoid:
      - "Do not parse opaque ids or retry a rejected mutation without first re-reading the authoritative revision."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
