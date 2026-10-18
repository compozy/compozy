# CH-workspace-registration-recovery: Register and correct project settings safely

```yaml
charter:
  id: CH-workspace-registration-recovery
  mission: "As Bruno, register and update a project through the public API, recover from invalid input, and read the same durable identity through the other transport."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-workspace-context
  scenarios: [RT-005, RT-007]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Register separate project directories over HTTP and UDS, then read each through the other transport."
      - "Try duplicate names and paths, relative root/additional directories, and an empty update name; compare saved state before and after refusals."
      - "Abandon a refused update, return with valid values, and confirm persistence after daemon restart."
    must_avoid:
      - "Registering the operator home or changing an existing real project."
```
