# CH-loop-quarantine-repair: Repair one sick Loop lane without losing healthy work

```yaml
charter:
  id: CH-loop-quarantine-repair
  mission: "As Ada, open one target breaker, preserve an independent healthy lane, diagnose the resulting quarantine, repair the target, and requeue the node through normal bounded succession."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-bound-runaway-work
  scenarios: [LP-sick-target-degrades-one-lane, LP-quarantine-diagnose-requeue, TA-loop-failure-breaker]
  tour: Recovery Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Fail one external target with transport errors while an independent target succeeds; only the sick family-target pair may open or fail fast."
      - "Repeat one node failure across generations; inspect the sanitized quarantine chain, timestamps, hints, target, and input reference through a public structured surface."
      - "Require the quarantined producer from pending work; needs-attention must name that producer without scheduling its consumers."
      - "Repair the target and requeue as an identified actor; verify requeue provenance, origin requeue, generation count equality, and final completion from a fresh read."
      - "Keep an unbounded failing watch as the terminal backstop control and a healthy watch as the non-tripping control."
    must_avoid:
      - "Reading SQLite or in-memory coordinator state as proof; use only CLI/HTTP/UDS and a separate fresh event or status read."
      - "Treating automated integration tests as a real-user QA verdict."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
