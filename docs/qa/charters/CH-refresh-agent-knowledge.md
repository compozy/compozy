# CH-refresh-agent-knowledge: Refresh an active agent's workspace knowledge

```yaml
charter:
  id: CH-refresh-agent-knowledge
  mission: "As Bruno, update one benchmark reference and use a documented Heartbeat wake to confirm that an active agent acts on the current value without another session prompt."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-refresh-agent-knowledge
  scenarios: [TA-agent-knowledge-refresh-on-wake]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Use a fresh isolated lab, a live native provider, and only documented CLI surfaces for the session and Heartbeat wake."
      - "Establish the original benchmark value, replace the Markdown file, then request exactly one synthetic wake without a second session prompt."
      - "Require the changed value within five minutes and confirm it independently through fresh session events and recap reads."
      - "Re-read Heartbeat status and session health, then execute the manifest teardown and cite clean evidence."
    must_avoid:
      - "Do not inspect internal databases, inject a second operator prompt, extend the five-minute deadline, or treat the separate autonomous completion bug as part of this verdict."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
