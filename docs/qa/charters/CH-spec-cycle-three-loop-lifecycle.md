# CH-spec-cycle-three-loop-lifecycle: The bundled package leaves and returns as one unit

```yaml
charter:
  id: CH-spec-cycle-three-loop-lifecycle
  mission: "Prove the current spec-cycle package disables and re-enables cleanly across every agent-manageable catalog surface."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [ET-052]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Capture the enabled extension, Loop, agent, and native-tool catalogs before mutation."
      - "Run `compozy extension disable spec-cycle`; confirm `implement-tasks`, `review-and-fix`, all four bundled agents, and every ext__spec_cycle__* tool leave together."
      - "Run `compozy extension enable spec-cycle`; confirm the same names return over CLI and HTTP without a watch-source kind."
      - "Compare the restored catalogs with the initial projection and record any ghost or missing entry."
    must_avoid:
      - "Starting a provider-backed Loop run; delegation behavior belongs to LP-implement-tasks-orchestrated-mode."
      - "Using the historical plural `compozy extensions` command."
  truthful_ui_check: "The visible Loop catalog contains exactly the two bundled spec-cycle Loops when enabled and none of them when disabled."
```

<!-- This charter is durable: re-run it in later cycles and append a fresh debrief per run. -->
