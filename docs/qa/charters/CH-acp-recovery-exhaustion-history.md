# CH-acp-recovery-exhaustion-history: Preserve history after bounded recovery is exhausted

```yaml
charter:
  id: CH-acp-recovery-exhaustion-history
  mission: "As Théo, let all three provider recovery attempts fail, then confirm one terminal failure leaves the original transcript readable and forkable."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: flaky
    locale: en-US
  journey: J-dead-session-history-recovery
  scenarios: [RT-acp-stream-disconnect-recovery]
  tour: Back-Button Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Trigger a provider that disconnects on the original prompt and all three replacement processes."
      - "Confirm three started events, one exhausted event, and one terminal provider-failure marker under the original turn."
      - "Reload the stopped session, navigate away and back, and read the same partial transcript and diagnostics without another automatic attempt."
      - "Fork a child in the same workspace and confirm parent provenance while the original remains unchanged."
    must_avoid:
      - "Do not edit the immutable earlier disconnect charter; this mission owns the new bounded-recovery behavior."
      - "Do not treat the expected three replacement failures as unrelated runtime errors."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
