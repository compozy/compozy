# CH-memory-batch-integrity: A memory batch commits whole or not at all

```yaml
charter:
  id: CH-memory-batch-integrity
  mission: "As Ada, abuse agh__memory_propose with ambiguous, duplicated, and mid-batch-failing operations and prove atomicity, deterministic rejection, prefix byte-stability, and next-session recall."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [MS-atomic-memory-batch]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Ordered add+replace+remove batch: no intermediate bytes visible, one decision recorded, identical retry reports already_applied."
      - "Missing and duplicated old_text → deterministic rejection naming the ambiguity, document and decision count unchanged; injected mid-batch failure → zero applied."
      - "At-capacity document: remove+add in one batch passes final-state validation in one round-trip."
      - "Three assembled prefix hashes before and after one committed write — identical until the write, changed exactly once after; next session recalls the committed fact."
    must_avoid:
      - "Editing memory files on disk — every mutation rides the native batch surface."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
