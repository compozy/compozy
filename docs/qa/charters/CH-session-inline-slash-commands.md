# CH-session-inline-slash-commands: Insert an effective command without losing the draft

```yaml
charter:
  id: CH-session-inline-slash-commands
  mission: "As Théo, use the session command menu while writing a real request and keep every surrounding word intact through selection, dismissal, and reload."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-use-session-slash-commands
  scenarios: [ET-session-slash-commands-inline]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open the menu with slash at the start, navigate its sections by keyboard, dismiss with Escape, and confirm the draft did not change."
      - "Write ordinary prose, type slash after whitespace in the middle, filter to a skill, select it, and confirm text before and after the query is byte-for-byte intact."
      - "Repeat with an emoji and accented text before the trigger, then refresh and return through the session deep link."
      - "At a narrow viewport, verify the menu stays inside the composer and every item remains readable and keyboard reachable."
    must_avoid:
      - "Do not enable or install skills during the session; catalog lifecycle mutation belongs to the structured catalog charter."
      - "Do not use evaluator language in the authored prompt."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
