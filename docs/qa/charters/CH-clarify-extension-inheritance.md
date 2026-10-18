# CH-clarify-extension-inheritance: An extension-asked question waits and answers like a native one

```yaml
charter:
  id: CH-clarify-extension-inheritance
  mission: "As Bruno, prove an extension-asked clarification inherits the same unbounded wait, keepalive ping, and answer contract as a native ask with no separate vocabulary."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-answer-agent-requests
  scenarios: [RT-session-clarification-roundtrip]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Trigger an extension-asked clarification under the unbounded policy; prove the same deadline-null pending, 30s pings, and late-answer resolution as a native ask."
      - "Repeat under a finite policy; expiry still falls back exactly once with timed_out and pings stop."
      - "Confirm the extension blocking caller simply waits longer under its own context, with no manifest, permission, or SDK change."
    must_avoid:
      - "Extension command discovery and exec policy — that surface is owned by J-run-extension-commands."
      - "Filing extension-host failures found off the clarify path; note them as follow-up charters instead."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's report. -->
