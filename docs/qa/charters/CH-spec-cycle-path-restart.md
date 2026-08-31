# CH-spec-cycle-path-restart: Resume implementation from task references

```yaml
charter:
  id: CH-spec-cycle-path-restart
  mission: "As Bruno, run a large authored task set through implement-tasks and keep every referenced task exact across a daemon restart."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-spec-cycle-path-fanout]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Import several large task files and confirm public tool output contains path and body_ref but no body field."
      - "Restart the daemon during fan-out, then confirm each worker reads the referenced path and completed lanes do not repeat."
      - "Fresh-read the terminal run through CLI and Web and compare task completion on disk."
    must_avoid:
      - "Injecting task content directly into the implementer prompt."
      - "Prompting a stalled worker to continue; a stall is a finding."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
