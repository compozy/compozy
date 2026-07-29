# CH-untested-051-complete-task-tree-bruno: Settle J-complete-task-tree for Bruno

```yaml
charter:
  id: CH-untested-051-complete-task-tree-bruno
  mission: "Walk J-complete-task-tree as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-task-tree
  scenarios: [TA-task-create-async-activation, TA-task-role-session-activation, TA-terminal-run-inspect, TA-web-task-detail-redesign, TA-web-task-run-detail-redesign]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Create a ready Task without waiting for worker provisioning; Activate a task-role worker after starvation recovery; Keep terminal task-run diagnostics terminal."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
