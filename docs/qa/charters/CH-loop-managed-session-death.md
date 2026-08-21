# CH-loop-managed-session-death: Prove one owner resumes a crashed Loop worker

```yaml
charter:
  id: CH-loop-managed-session-death
  mission: "As Ada, crash a Loop worker session and prove only the Loop lifecycle owner resumes or parks the node."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: flaky
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [RT-subprocess-health-escalation, LP-crash-death-resume]
  tour: Interrupt Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Kill the managed process after a checkpoint and compare task, Loop node, and event history through public reads."
      - "Confirm exactly one continuation and no generic subprocess needs-attention transition for the Loop worker."
      - "Repeat at the death-streak boundary and verify progress resets the streak."
    must_avoid:
      - "Equating missed health checks with confirmed process death."
      - "Injecting state directly into the database."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
