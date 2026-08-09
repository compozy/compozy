# CH-gateway-ssh-ownership: Interrupt SSH without leaking ownership

```yaml
charter:
  id: CH-gateway-ssh-ownership
  mission: "As Bruno, run the Interrupt Tour through SSH start, reuse, local forwarding, accepted remote work, parent death, and disconnect, and prove Compozy closes only the daemon, lease, and forward that connection owns."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: flaky
    locale: en-US
  journey: J-connect-gateway-ssh
  scenarios: [RT-gateway-ssh-forward, RT-gateway-local-only-boot]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Fail SSH authentication, changed-host-key verification, remote binary discovery, and version checks; each refusal must happen before a daemon or forward is left behind."
      - "Connect once to start an owned daemon and again to reuse it; inspect busy fencing, leases, requested remote home, and loopback-only listeners."
      - "Kill the tunnel and parent after work is accepted, reconnect, and confirm the work continued while orphaned owned resources were reaped."
      - "Disconnect both connections in different orders; a reused daemon and unrelated SSH control master must survive, while the last owned daemon stops."
    must_avoid:
      - "Relaxing known-host verification; enabling a gateway provider or tier to make the SSH path pass."
  evidence_expectations:
    - "Process/socket inventories before and after each interruption, lease ownership, daemon home and version, accepted work receipt, and clean scoped teardown."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->

