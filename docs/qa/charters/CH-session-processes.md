# CH-session-processes: Retain and clean up background work

```yaml
charter:
  id: CH-session-processes
  mission: "As Théo, control the background commands a session leaves running, including after daemon restart."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-session-processes
  scenarios: [RT-session-processes]
  time_box_minutes: 60
  guidance:
    must_try:
      - "Inspect sleep and HTTP server identities and the server's port."
      - "Stop with terminate and independently confirm exit."
      - "Stop with keep, restart the isolated daemon, and terminate the retained orphans."
      - "Observe native terminal shutdown as an adjacent lifecycle canary."
    must_avoid:
      - "Restarting the operator daemon or signaling unrelated OS processes."
```
