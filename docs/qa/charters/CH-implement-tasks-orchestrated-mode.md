# CH-implement-tasks-orchestrated-mode: Delegate one bounded worker per task

```yaml
charter:
  id: CH-implement-tasks-orchestrated-mode
  mission: "As Bruno, run implement-tasks in orchestrated mode and prove the conductor only delegates, category runtimes reach workers, task files prove completion, and every worker stops."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-implement-tasks-orchestrated-mode]
  tour: Feature Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Start implement-tasks with slug, mode=orchestrated, and explicit orchestrator/backend/frontend/default runtime inputs."
      - "Confirm the bundled orchestrator conducts one continuous Goal session and never edits production files itself."
      - "Watch one bounded worker per task receive its category runtime, then independently read completed frontmatter from disk."
      - "Confirm every spawned worker is stopped, the per-task path is not_taken, and the Run settles done."
    must_avoid:
      - "Accepting the conductor report without task-file and session-list proof."
      - "Querying SQLite or internal handlers as pass evidence."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
