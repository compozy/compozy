# CH-automation-manual-trigger: Start one job immediately

```yaml
charter:
  id: CH-automation-manual-trigger
  mission: "As Bruno, manually trigger a workspace-owned automation job through Web, HTTP, and CLI and prove every surface returns the same newly started run."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [TA-053]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create a task-backed job, trigger it once through each public entry point, and inspect the returned run plus job history."
      - "Attempt a missing job and a job from another workspace; both must fail without creating a run."
      - "Reload the Web detail and rediscover each manual run from daemon-owned history."
    must_avoid:
      - "Using provider-prompt success as the trigger contract or accepting a response without proving workspace ownership and run persistence."
```
