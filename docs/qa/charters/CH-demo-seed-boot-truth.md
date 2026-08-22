# CH-demo-seed-boot-truth: Recreate the demo world and trust what survives boot

```yaml
charter:
  id: CH-demo-seed-boot-truth
  mission: "As Dora, recreate the Northstar Pay demo world and verify that daemon boot preserves its truthful history and populated operator surfaces."
  mode: strategy-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-prepare-demo-recording
  scenarios: [RT-demo-seed-replace-boot]
  tour: Data Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Seed once, seed twice with replace, and compare the reported and independently read counts."
      - "Start the real daemon, inspect Loop history through HTTP, and verify that reconciliation does not promote, cancel, or rewrite imported rows."
      - "Inspect Goal turns, the linked Git worktree, memory, task outcomes, and workspace isolation through public read surfaces."
      - "Attempt replace against an unowned directory and confirm the command refuses it without deleting content."
    must_avoid:
      - "Do not decide the verdict from direct SQLite inspection alone; use it only as an independent confirmation after public-surface reads."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
