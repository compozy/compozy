# CH-clarify-long-wait-keepalive: A clarification asked now still answers after the agent's idle limit

```yaml
charter:
  id: CH-clarify-long-wait-keepalive
  mission: "As Théo, hold one unbounded clarification past the 60s mark, prove keepalive pings kept the agent call alive, and take the late answer — then run the finite control that must still fall back."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-answer-agent-requests
  scenarios: [RT-session-clarification-roundtrip]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Ask under the unbounded policy; prove pending shows deadline null with no countdown on CLI -o json, wait past the 60s mark, and confirm _compozy/clarify_ping seq counting from 1 in daemon debug logs keyed by session/request ID."
      - "Answer late (--choice 1 selects the first choice, which the blocked call receives as choice 0 with fallback false) and prove the agent call returns the exact answer, never the fallback sentinel."
      - "Run the finite control: no answer within the policy still yields the fallback sentinel with timed_out, and a timely answer resolves normally."
      - "Record the agent used and whether its idle limit needed the 30s margin; a non-supporting agent keeping its own timeout is documented behavior, not a defect."
    must_avoid:
      - "Claiming E2E-001/E2E-002 as automated evidence — this is a live operator walk that corroborates them."
      - "Judging web rendering — zero-deadline display is owned by .compozy/tasks/clarify-timeout, not this spec."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's report. -->
