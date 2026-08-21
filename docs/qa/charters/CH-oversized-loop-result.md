# CH-oversized-loop-result: Reject a result above the Loop contract

```yaml
charter:
  id: CH-oversized-loop-result
  mission: "As Bruno, return an oversized Loop action result and prove validation failure settles every owner cleanly."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-oversized-action-result-fails]
  tour: Garbage Tour
  time_box_minutes: 25
  guidance:
    must_try:
      - "Return payloads immediately below and above 64 KiB."
      - "Compare task, task run, Loop node, and lease state through CLI, HTTP, UDS, and native reads."
      - "Refresh after settlement and confirm no success output or running lease survives."
    must_avoid:
      - "Reading internal storage as the primary verdict."
      - "Using a provider response whose size cannot be reproduced exactly."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
