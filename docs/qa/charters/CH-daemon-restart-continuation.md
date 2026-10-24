# CH-daemon-restart-continuation: Keep sessions running across a CompozyOS restart

```yaml
charter:
  id: CH-daemon-restart-continuation
  mission: "As Dora, restart a busy runtime and verify that safe work continues once while uncertain effects and explicit stops remain under operator control."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-daemon-restart-continuation
  scenarios: [RT-daemon-restart-continuation, RT-subagent-restart, RT-daemon-drain-admission]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Restart three streaming sessions normally and through SIGKILL; verify retained partial text, one continuation per source turn, and ordered queued input."
      - "Repeat restarts around continuation admission and completion; check the durable breaker and unknown-outcome tool handling."
      - "Exercise pending approvals, clarification, changed runtime or permissions, opt-out, and explicit stop."
      - "Compare fresh Web, CLI and HTTP/UDS state, then verify all isolated processes are cleaned up."
    must_avoid:
      - "Never stop the operator's real daemon or repeat an external effect merely because its result was interrupted."
```
