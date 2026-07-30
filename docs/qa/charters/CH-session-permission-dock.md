# CH-session-permission-dock: Decide at the composer and keep a receipt

```yaml
charter:
  id: CH-session-permission-dock
  mission: "As Théo, resolve queued permissions and clarifications at the composer by pointer and keyboard, then verify that every outcome leaves one truthful transcript receipt."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-answer-agent-requests
  scenarios: [ET-web-session-permission-dock]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Exercise all offered permission decisions, including shortcut 4 while its split menu is closed and a focused-input shortcut negative."
      - "Answer choice and free-text clarifications, including Enter, Shift+Enter, retryable error, timeout, and reload while pending."
      - "Queue multiple decisions and prove permissions lead, the 1/N counter advances, and both allowed and rejected receipts persist."
    must_avoid:
      - "Triggering digit shortcuts from a focused input or treating a visual dismissal as proof that the runtime recorded a decision."
```
