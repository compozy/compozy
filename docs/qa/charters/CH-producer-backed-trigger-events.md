# CH-producer-backed-trigger-events: Trust producer-backed trigger events

```yaml
charter:
  id: CH-producer-backed-trigger-events
  mission: "As Bruno, create and update triggers across the public operator surfaces, abuse the event field, then cause real producer actions and prove only valid, matching triggers record runs."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-create-and-activate-trigger
  scenarios: [TA-057, TA-056]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create and update valid built-in, hook, webhook, and ext.* definitions; independently read each stored event."
      - "Try an unknown event, whole-event whitespace, hook delimiter padding, a bare ext. prefix, and the valid free-form ext. release suffix."
      - "Run config validate with valid and invalid trigger definitions and inspect the structured error path."
      - "Cause at least one real lifecycle producer and a successful workspace memory consolidation, then read trigger history from a second public surface."
    must_avoid:
      - "Calling internal observers or reading SQLite to prove activation."
      - "Treating a stored trigger as proof that its producer fired."
```
