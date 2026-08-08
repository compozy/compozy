# CH-visible-session-window-streams: Two visible session windows keep streaming across focus changes

```yaml
charter:
  id: CH-visible-session-window-streams
  mission: "As Théo, watch two active sessions side by side, move focus between them, then hide and restore one window so visible streams stay current and hidden streams catch up."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-13
  scenarios: [RT-visible-session-streaming]
  tour: Multi-Tab Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Tile two active session windows side by side and move focus while both agents emit distinct delayed messages."
      - "Minimize one window, wait for another message, restore it, and confirm catch-up without reloading."
      - "Repeat the hidden branch with an inactive desktop or inactive stack when reachable."
    must_avoid:
      - "Do not keep refocusing the first window to make its transcript advance."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
