# CH-orchestrate-tasks-delegation: Deliver an authored spec through one worker session per task

```yaml
charter:
  id: CH-orchestrate-tasks-delegation
  mission: "As Bruno, run the bundled orchestrate-tasks Loop against an authored slug and prove the conducting Goal session only delegates: one named worker session per task, stopped on every path, with completion read from the task files."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-orchestrate-tasks-delegation]
  tour: Feature Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Start the Loop from the documented CLI with only slug, and confirm the orchestrator input resolves the default general agent."
      - "Watch the spawned sessions live: exactly one active orchestrate-<slug>-<task_id> worker at a time, and none left active after each task settles."
      - "Leave one task file at status pending and read the rejected tasks_completed verdict, then complete it and watch the Run advance to done."
      - "Confirm the Loop pins no provider or model and that the published catalog entry distinguishes it from implement-tasks."
    must_avoid:
      - "Do not accept the orchestrator's own report as completion evidence; read the task frontmatter."
      - "Do not query SQLite or use an internal handler as pass evidence."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
