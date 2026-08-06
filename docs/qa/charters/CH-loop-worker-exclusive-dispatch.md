# CH-loop-worker-exclusive-dispatch: Keep one runtime owner per Loop worker

```yaml
charter:
  id: CH-loop-worker-exclusive-dispatch
  mission: "As Bruno, run adjacent Loop-owned and ordinary task-role work and prove each receives exactly one owner, one initial prompt, and one completion path."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-task-tree
  scenarios: [TA-loop-worker-exclusive-dispatch, TA-task-role-session-activation]
  tour: Feature Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Start one Loop whose action creates a worker run, then observe its task run and session through public run and session surfaces."
      - "Let scheduler recovery inspect the live worker and confirm no coordinator or ordinary task-role session appears for it."
      - "Run one adjacent ordinary task-role assignment and confirm starvation recovery still activates it exactly once."
      - "Refresh both task-run and session reads after completion; each run must retain one owner, one initial prompt, and one terminal result."
    must_avoid:
      - "Using coordinator memory, direct store queries, or internal test hooks as session evidence."
      - "Treating the absence of a duplicate at one instant as proof; wait through a scheduler cycle and re-read."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
