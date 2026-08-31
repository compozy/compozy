# CH-implement-tasks-orchestrated-mode: Delegate one bounded worker per task

```yaml
charter:
  id: CH-implement-tasks-orchestrated-mode
  mission: "As Bruno, run implement-tasks in orchestrated mode with a selected workspace Agent and prove the conductor only delegates, the Agent-local skill and category runtimes reach workers, task files prove completion, and every worker stops."
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
      - "Create a non-default workspace Agent with an Agent-local sentinel skill, then start implement-tasks with slug, mode=orchestrated, implementer=custom_implementer, and explicit orchestrator/backend/frontend/default runtime inputs."
      - "Confirm the bundled orchestrator conducts one continuous Goal session and never edits production files itself."
      - "Confirm every bounded worker reports agent_name=custom_implementer, sees the Agent-local sentinel, and receives its category runtime; then independently read completed frontmatter from disk."
      - "Confirm every spawned worker is stopped, the per-task path is not_taken, and the Run settles done."
    must_avoid:
      - "Accepting the conductor report without task-file and session-list proof."
      - "Querying SQLite or internal handlers as pass evidence."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
