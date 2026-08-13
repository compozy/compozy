# CH-loop-await-child-recovery: Preserve awaited child Loop progress through restart

```yaml
charter:
  id: CH-loop-await-child-recovery
  mission: "As Ada, run two dependent awaited child Loops and interrupt the daemon while the first child is live so that I can trust the parent neither finishes early nor creates duplicate child work."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [LP-run-loop-await-child]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Use only structured CLI entry points to validate and start the parent and to inspect the parent and child states."
      - "Restart the isolated daemon while the first awaited child is still live, then read the parent again before the child settles."
      - "Confirm the authored successor starts once only after the first child reaches its accepted terminal state."
    must_avoid:
      - "Using SQLite, internal debug endpoints, or direct store edits as verdict evidence."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report, never here. -->
