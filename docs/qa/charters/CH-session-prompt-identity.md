# CH-session-prompt-identity: Does one authored prompt remain one durable message across retry and reload?

```yaml
charter:
  id: CH-session-prompt-identity
  mission: "As Théo, send one identified prompt, observe it settle live, retry the exact command, and reload the permalink without duplicating the message or its effect."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [RT-session-prompt-idempotency, RT-session-message-reload]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Send a uniquely named prompt in the Web thread and observe its optimistic row settle against the live transcript exactly once."
      - "Repeat the same public command with its original message_id and idempotency_key, then independently read the transcript and receipt."
      - "Reload the exact permalink and confirm the authored text remains once, in its original chronology."
      - "Reuse the idempotency key with changed text and the message id with a different key; both must fail with distinct structured conflicts."
    must_avoid:
      - "Using SQLite as persona evidence, accepting an optimistic row as durable proof, or changing identities during an exact retry."
  evidence_expectations:
    - "Web screenshot after live settlement and after cold reload, plus structured HTTP or CLI request/response evidence for replay and conflicts."
    - "Independent transcript read proving one authored message id and no second provider-visible effect."
  truthful_ui_check: "The first stream update must not duplicate or move the authored row, and an exact retry must not create a second stream or message."
```
