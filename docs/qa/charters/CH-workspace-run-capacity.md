# CH-workspace-run-capacity: Drain queued work within a workspace execution bound

```yaml
charter:
  id: CH-workspace-run-capacity
  mission: "As Ada, operate two workspaces at a one-run bound and prove deferred work stays durable, isolated, and drains when capacity opens."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-bounded-task-capacity
  scenarios: [TA-workspace-run-capacity]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Set the limit through a structured config surface, confirm restart-required truth, restart, and read the active value."
      - "Race two workspace-A claims, then compare non-waiting typed conflict with waiting poll behavior and durable queue state."
      - "While A is full, claim workspace-B, global, and Network wake work; none may consume or inherit A's limit."
      - "Open capacity by completion, release, and lease expiry; each deferred run must progress without manual re-enqueue."
    must_avoid:
      - "Do not use Web UI evidence; this milestone adds no Web control and is verified through structured surfaces."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
