# CH-wake-dedup-stress: One wake delivers once, even after eviction and restart

```yaml
charter:
  id: CH-wake-dedup-stress
  mission: "As Ada, stress task-creator wake dedup with a large decoy event history, cache eviction, and a daemon restart, proving one delivery per wake identity with no false suppression."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-bounded-task-capacity
  scenarios: [TA-task-wake-dedup]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Repeat one wake_event_id across cache eviction and a daemon restart → delivered exactly once; the audit rows show delivered vs suppressed."
      - "Pad the task with a large unrelated event history → the authoritative lookup stays indexed and task-scoped (no cost growth, no cross-task suppression)."
      - "Same identity on a different task and a distinct identity on the same task → both deliver."
    must_avoid:
      - "Truncating the event ledger to make room — the large-history case is the point."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
