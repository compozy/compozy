# CH-crash-resume-compaction: Kill the daemon at every ugly moment and lose no context

```yaml
charter:
  id: CH-crash-resume-compaction
  mission: "As Théo, drive a long session past the replay budget and to 0.95 of the context window, kill the daemon before a clean session end, and prove degraded resume rebuilds one bounded replay (pinned first message, omission note, workspace-authority line, history pointer) while CompozyOS itself never compacts, archives, or starts a child session — and an observed agent compaction is recorded exactly once, with no cross-workspace bleed."
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
      - "Kill mid-conversation with a load-unsupported provider fixture; on resume, ask for the pinned first-message fact and for a fact inside the protected last 8 messages, and require the 'Context rebuilt from log.' marker plus both answers (timestamped kill/resume commands)."
      - "Push usage to 0.95 of the window after complete turns: no child session, no archived row, no session.compaction_fired, no hook dispatch. Then replay the recorded Claude compaction frames and kill the daemon between the terminal frame and the next usage report: after restart there is still exactly one Compaction item, one event, and the context reading is unknown."
      - "Run the same content in a second workspace throughout — no replay row or fact may cross workspaces, and no project_checkpoint_summary.md may appear in either."
      - "Control runs: successful session/load performs no replay and adds no marker; an agent that does not advertise the compaction capability produces no Compaction item, event, marker, or hook call."
    must_avoid:
      - "Editing event stores by hand; every proof rides public surfaces plus DB dumps."
      - "Sampling one crash window only — the gap between the terminal compaction frame and the next usage report is mandatory."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->

<!-- 2026-10-07 (memory removal): mission and guidance rewritten for bounded rebuilds and observed agent compaction; the scenario list is unchanged. -->
