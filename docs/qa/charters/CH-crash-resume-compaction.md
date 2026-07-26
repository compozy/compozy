# CH-crash-resume-compaction: Kill the daemon at every ugly moment and lose no context

```yaml
charter:
  id: CH-crash-resume-compaction
  mission: "As Théo, drive a long session through compaction pressure, kill the daemon before a clean session end, and prove degraded resume reconstructs every archived fact from the checkpoint summary — no silent loss, no re-inflation, no cross-workspace bleed."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [RT-session-context-rebuild, RT-pressure-context-compaction, MS-workspace-checkpoint-continuity]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Kill mid-conversation with a load-unsupported provider fixture; on resume, ask for a unique pre-restart fact and require the 'Context rebuilt from log.' marker plus the answer (timestamped kill/resume commands)."
      - "Push usage past the compaction threshold after complete turns; verify summary-before-archive ordering, archived rows still queryable, and an interrupt between coverage and archive retried idempotently."
      - "Run the same content in a second workspace throughout — no checkpoint fact or replay row may cross workspaces."
      - "Control runs: successful session/load performs no replay and adds no marker; pressure_threshold=0 dispatches no hooks."
    must_avoid:
      - "Editing event stores by hand; every proof rides public surfaces plus DB dumps."
      - "Sampling one crash window only — the coverage-vs-archive gap window is mandatory."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
