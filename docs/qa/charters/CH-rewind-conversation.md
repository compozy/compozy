# CH-rewind-conversation: Correct a mistaken session path without losing its useful prefix

```yaml
charter:
  id: CH-rewind-conversation
  mission: "Prove that an operator can rewind an idle conversation in place, understand the side-effect boundary, and continue from the retained prefix across Web and structured surfaces."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-rewind-conversation
  scenarios: [RT-conversation-rewind]
  tour: data-tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create two distinct prompt paths, rewind from the second durable user message, and edit the restored draft."
      - "Refresh the Web session and independently inspect active and archived history from the CLI or API."
      - "Cancel the confirmation once and verify that no transcript or composer state changes."
    must_avoid:
      - "Do not treat filesystem, tool, memory, or network side effects as part of rewind."
      - "Do not use internal database inspection as pass evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run report. -->
