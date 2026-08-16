# CH-bounded-runtime-identity: Keep daemon liveness bounded under status load

```yaml
charter:
  id: CH-bounded-runtime-identity
  mission: "As Ada, repeatedly bind to one live daemon through HTTP and UDS while full status is read in parallel, proving liveness stays bounded and the diagnostic snapshot stays truthful."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-daemon-schema
  scenarios: [RT-bounded-runtime-identity, RT-001]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Read `/api/status/identity` repeatedly over HTTP and UDS; confirm every response names the same process, listener, home, build, and schema."
      - "Read `/api/status` before, during, and after the identity burst; confirm its complete sections remain available and unchanged in shape."
      - "Compare response sizes and elapsed time, then request an unknown identity subpath and confirm a 404 without affecting later reads."
      - "Capture the Rust transport regression and Go route tests alongside the live public-surface evidence."
    must_avoid:
      - "Do not use the identity response as a replacement for status or doctor diagnostics."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
