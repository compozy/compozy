# CH-loop-terminal-time-recovery: Read exact completion and isolate bad history

```yaml
charter:
  id: CH-loop-terminal-time-recovery
  mission: "As Ada, follow a Loop from live to terminal and through restart or rerun, proving its completion time is durable and one invalid snapshot cannot block healthy recovery."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-loop-terminal-recovery
  scenarios: [LP-terminal-completion-time, LP-invalid-snapshot-boot-isolation]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Compare completed_at with the durable terminal event across CLI, HTTP, native tools, and the Web duration after refresh."
      - "Rerun the terminal run and confirm completed_at is absent while it is live."
      - "Restart with one intentionally invalid persisted snapshot and one healthy run, if an operator-owned corruption fixture is available."
    must_avoid:
      - "Claiming snapshot isolation passed without a public corruption path or leaving the QA lab alive after a blocked result."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
