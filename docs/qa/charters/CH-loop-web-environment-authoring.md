# CH-loop-web-environment-authoring: Web narrows choices without losing config

```yaml
charter:
  id: CH-loop-web-environment-authoring
  mission: "As Bruno, edit Loop environments in Web and prove advanced values created through the API or CLI remain visible and unchanged."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-isolated-task-loop-execution
  scenarios: [LP-worktree-web-loop-environment]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Confirm configure, run, and node controls offer only Inherit, Workspace root, and Named worktree for new choices."
      - "Load directory and per-run values created through a structured surface; confirm their read-only labels and exact values."
      - "Save or publish an unrelated edit, reload through an independent read, and confirm the advanced environment value is byte-for-byte unchanged."
    must_avoid:
      - "Treating optimistic UI state or component-test mocks as saved-state evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
