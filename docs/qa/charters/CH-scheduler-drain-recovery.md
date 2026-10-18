# CH-scheduler-drain-recovery: Drain and resume dispatch with truthful bounds

```yaml
charter:
  id: CH-scheduler-drain-recovery
  mission: "As Dora, drain scheduler dispatch, distinguish completion from timeout, and restore dispatch through a fresh public client."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-drain-scheduler
  scenarios: [TA-048]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Compare status and drain through CLI, HTTP, and UDS, including zero timeout and invalid bounds."
      - "Leave after draining and return through another client; resume and independently read paused=false."
      - "Discover the advertised Web entry and record a missing or unreachable affordance honestly."
    must_avoid:
      - "Treating daemon admission drain as scheduler drain, or canceling admitted work to force completion."
```
