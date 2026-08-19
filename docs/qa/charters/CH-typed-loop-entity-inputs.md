# CH-typed-loop-entity-inputs: Select exact Loop entities across public start surfaces

```yaml
charter:
  id: CH-typed-loop-entity-inputs
  mission: "As Lea, start one Loop with exact typed entities and a partial runtime while proving stale values fail safely and consistently across public surfaces."
  mode: charter-with-tour
  persona:
    name: Lea
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-select-typed-loop-entities]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Compare enum, agent, skill, Loop, worktree, session, workspace, secret, runtime, and file fields in the Web run form."
      - "Submit exact catalog values and exact manual values through CLI, HTTP/UDS, and compozy__loop_run."
      - "Use a human TTY with a missing supported required value, then repeat with --no-prompt and structured output."
      - "Make one reference stale and confirm input_validation names its field and origin while no run, task, or ACP session appears."
    must_avoid:
      - "Reading secret values or treating display labels as stored references."
      - "Claiming file browsing or existence validation."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
