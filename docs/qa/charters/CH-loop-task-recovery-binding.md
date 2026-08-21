# CH-loop-task-recovery-binding: Recover the exact Loop task cell once

```yaml
charter:
  id: CH-loop-task-recovery-binding
  mission: "As Bruno, recover one Loop-owned needs-attention task and prove the continuation replaces the exact durable cell without duplicate or orphaned work."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [TA-033]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Recover through CLI, then compare HTTP and UDS fresh reads for one failed source and one queued continuation."
      - "Verify generation, node id, and item index stay fixed while attempt and epoch advance exactly once."
      - "Repeat the recovery request and confirm it cannot create a second continuation."
    must_avoid:
      - "Using direct SQLite reads as the verdict."
      - "Treating a new run id alone as proof that the correct Loop cell advanced."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
