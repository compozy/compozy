# CH-task-tree-loop-rollup: Finish delegated work and observe the automatic follow-up

```yaml
charter:
  id: CH-task-tree-loop-rollup
  mission: "As Bruno, create a parent with two children, arm a Loop for the parent transition, complete the children, and trust the parent plus one follow-up task to settle automatically."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-task-tree
  scenarios: [TA-parent-rollup-completion, LP-task-rollup-wakes-loop, LP-042]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Use only task and Loop UI flows for creation/status changes; independently verify with a public structured read."
      - "After child A completes, confirm the parent remains non-terminal."
      - "After child B completes, confirm the parent completes exactly once and the Loop creates exactly one follow-up."
      - "Repeat the final transition with the Loop disabled and with a neighboring workspace tree."
```
