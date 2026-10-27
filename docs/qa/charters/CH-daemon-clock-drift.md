# CH-daemon-clock-drift: Stop the same runtime after a long-running workday

```yaml
charter:
  id: CH-daemon-clock-drift
  mission: "As a CLI operator, start my runtime, recognize a duplicate start, restart it, and stop it without touching another home."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  scenarios: [RT-daemon-clock-drift]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start, read status, attempt another start, restart, read status, and stop."
      - "On WSL2, repeat after host sleep and verify the existing PID remains manageable."
    must_avoid:
      - "The operator's default home, unrelated daemons, and provider sessions."
```
