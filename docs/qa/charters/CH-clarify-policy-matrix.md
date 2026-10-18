# CH-clarify-policy-matrix: The clarification timeout policy matrix holds across restarts and rejects garbage

```yaml
charter:
  id: CH-clarify-policy-matrix
  mission: "As Dora, prove omitted, 0s, finite, and invalid tools.clarify.timeout values resolve to the effective policy the daemon advertises, apply only after restart, and never replace the last valid policy on rejection."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-runtime-settings
  scenarios: [MS-clarify-timeout-policy]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Set omitted, 0s, 5m, and the 1s/24h bounds through compozy config set; restart and prove the effective value and restart-required lifecycle agree across CLI structured output, the config file, and the pending projection (deadline null when unbounded)."
      - "Apply -5s, 500ms, 25h, soon, and 99h; each must fail with the exact tools.clarify.timeout error while the last valid policy stays active and effective."
      - "Flip the live policy mid-wait and prove the pending request keeps its creation-time deadline."
    must_avoid:
      - "Answering clarifications to settle runtime verdicts — waits, pings, and races belong to the keepalive charters."
      - "Inferring a verdict from source, mocks, or automated suites; every claim needs a fresh structured read."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's report. -->
