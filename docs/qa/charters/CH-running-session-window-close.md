# CH-running-session-window-close: Close and reopen a live session window cleanly

```yaml
charter:
  id: CH-running-session-window-close
  mission: "As Théo, close a session window during a real running turn and reopen it without an error boundary, a product console error, or lost background work."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [ET-running-session-window-close-clean]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Close the window while output is still streaming and inspect the visible desktop plus product console."
      - "Reopen the same session from the product navigation and confirm the current transcript and live state render."
      - "Repeat once through a normal route transition to exercise Strict Mode teardown without duplicate close effects."
    must_avoid:
      - "Killing the ACP process or daemon; the interruption is the window lifecycle only."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
