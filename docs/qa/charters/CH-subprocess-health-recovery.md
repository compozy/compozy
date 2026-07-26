# CH-subprocess-health-recovery: A dying subprocess is seen, parked once, and deliberately recovered

```yaml
charter:
  id: CH-subprocess-health-recovery
  mission: "As Ada, crash and degrade a task-bound ACP subprocess and prove health evidence agrees across every structured surface, the exact linked run parks needs_attention exactly once, terminal state always wins, and recovery is deliberate."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-diagnose-task-session-health
  scenarios: [RT-subprocess-health-escalation]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Failed health verdicts: identical bounded evidence via HTTP, UDS, agh status, and doctor --only runtime.subprocess_health."
      - "Cross the configured threshold → the exact linked nonterminal run reaches needs_attention once (one canonical event) under repeated checks; an unexpected process exit escalates immediately."
      - "Terminal runs never mutate; threshold 0 keeps diagnostics without task mutation (restart-required config)."
      - "Recover the parked run only after repairing the provider cause; fresh reads must agree with one correlated escalation and one deliberate continuation."
    must_avoid:
      - "Expecting an automatic subprocess restart — none exists in this program."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
