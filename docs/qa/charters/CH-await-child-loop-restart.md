# CH-await-child-loop-restart: Does an awaited child survive interruption without reordering work?

```yaml
charter:
  id: CH-await-child-loop-restart
  mission: "As Bruno, run two ordered child Loops and interrupt the daemon while the first is live to prove the parent resumes the same work before advancing."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-await-child-loop
  scenarios: [LP-run-loop-await-child-ordering]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Read the live parent through structured CLI and HTTP/UDS surfaces before and after restart."
      - "Compare the exact first-child identity across restart and count child runs before releasing it."
      - "Release each durable wait separately and confirm the second child begins only after the first is terminal."
      - "Re-read the terminal parent and both children through an independent public surface."
    must_avoid:
      - "Extensions, agents, providers, native tools, or application-specific orchestration."
```

<!-- The charter is durable and immutable; run debriefs belong in dated reports. -->
