# CH-session-goal-strip: Control a Goal from its compact session surface

```yaml
charter:
  id: CH-session-goal-strip
  mission: "As Bruno, read a Goal at a glance, expand its honest details, and use exactly the lifecycle action the current state permits without sending a staged command accidentally."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-26
  scenarios: [ET-web-session-goal-strip]
  tour: Feature Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Walk active, needs-approval, paused, terminal, moved, unknown-context, and failed-read states with reloads between transitions."
      - "Expand the strip and compare objective, run/session links, turn, context, verdict, node, and cause with fresh public reads."
      - "Stage draft and replacement commands, then prove the composer does not send until the operator submits."
    must_avoid:
      - "Inferring Goal state from color alone or accepting an invented context percentage while the read is pending."
```
