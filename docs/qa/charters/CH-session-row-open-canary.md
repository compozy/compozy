# CH-session-row-open-canary: Open a session normally after row actions change

```yaml
charter:
  id: CH-session-row-open-canary
  mission: "As Nia, open an ordinary session from its catalog row and a direct link after the row actions change, then prove navigation and the fresh snapshot still identify the same session without opening the action menu."
  mode: charter-with-tour
  persona:
    name: Nia
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-12
  scenarios: [RT-012]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open a normal session by clicking the row body, reload the canonical route, and compare the visible id/title with HTTP and UDS status."
      - "Open the three-dot menu with pointer and keyboard, dismiss it with Escape, and confirm neither interaction navigates."
      - "Open the same session by direct link and confirm archived_at is null and the normal detail controls remain available."
    must_avoid:
      - "Long-transcript performance and reconnect paths owned by CH-015 and CH-014."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
