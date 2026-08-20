# CH-acp-tool-update-burst: A headless agent completes a tool-update burst without losing the prompt

```yaml
charter:
  id: CH-acp-tool-update-burst
  mission: "As Ada, send one prompt through a provider that repeats an in-progress tool update more than 1,024 times and prove the structured stream remains canonical and reaches prompt completion."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-acp-tool-update-burst]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Follow the prompt through the public structured surface with a two-event prompt buffer and a provider subprocess that emits more than 1,024 identical in-progress updates for one tool."
      - "Confirm the stream contains the first tool call, every meaningful title/kind/input enrichment once, the terminal result, and prompt completion in that order."
      - "Send a follow-up prompt through the same connected provider process to prove the burst did not terminate the session."
    must_avoid:
      - "Do not increase the notification or prompt buffer; the mission verifies bounded behavior at the semantic owner."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
