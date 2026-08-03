# CH-agent-loop-lifecycle-native: Operate Loop lifecycle without a UI

```yaml
charter:
  id: CH-agent-loop-lifecycle-native
  mission: "As Ada, discover and use every Loop lifecycle native tool, then prove fresh state, deterministic loser answers, and workspace isolation without a web UI."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-07
  scenarios: [LP-agent-operates-lifecycle-via-native-tools, TA-076]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Discover the exact eight lifecycle descriptors and confirm compozy__loop_stop is unknown."
      - "Invoke run cancel/kill, node pause/resume/cancel/kill/requeue, and the paginated node inventory through native tools only."
      - "Repeat and race incompatible verbs; compare deterministic actual_state, allowed_transitions, and winner provenance with a fresh HTTP or CLI read."
      - "Query workspace A from an A-scoped agent and prove no workspace-B node is returned or accepted."
    must_avoid:
      - "Using database reads or internal Go calls as proof of user-visible behavior."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
