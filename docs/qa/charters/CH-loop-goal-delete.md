# CH-loop-goal-delete: Run, revise, and remove a custom Loop

```yaml
charter:
  id: CH-loop-goal-delete
  mission: "As Bruno, fork a Loop, add a concrete goal and definition-of-done, publish and run it, remove the optional goal and run again, then intentionally delete only the custom fork."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-06
  scenarios: [LP-toggle-loop-goal, LP-delete-custom-loop]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Use the editor and publish modals, then verify each saved contract after a reload."
      - "Give the goal-bearing run a real coding task and verify the run's Goal/DoD UI matches the saved version."
      - "Remove the goal, publish, and prove the next run has no stale Goal chip or judge state."
      - "Delete from the destructive-action modal; cancel once before confirming; verify the built-in source remains."
```
