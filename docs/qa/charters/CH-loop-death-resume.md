# CH-loop-death-resume: Kill the managed session and prove the Loop resumes once

```yaml
charter:
  id: CH-loop-death-resume
  mission: "As Ada, kill a checkpointing Loop node's managed session at difficult boundaries and prove the durable authority resumes it exactly once or parks it with an honest reason."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: flaky
    locale: en-US
  journey: J-resume-dead-loop-node
  scenarios: [LP-crash-death-resume]
  tour: Recovery Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Kill after a checkpoint, before a checkpoint, and twice before the first detector settles; each confirmed death may reserve at most one continuation."
      - "Race node cancellation with death detection; cancellation must win with no new binding or continuation."
      - "Park the node before killing its session; parked work must not resume."
      - "Make durable progress between deaths to reset the streak, then inject three deaths without progress and inspect resume_exhausted attention."
    must_avoid:
      - "Treating silence or a missed heartbeat as confirmed death."
      - "Reading SQLite directly as the verdict; use public structured reads and event history."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
