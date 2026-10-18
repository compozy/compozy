# CH-browser-multitab-stream-continuity: Keep commands and background updates working across tabs

```yaml
charter:
  id: CH-browser-multitab-stream-continuity
  mission: "As Théo, open several browser documents, send useful work to a real session, and stop it without losing history or blocking navigation."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-13
  scenarios: [RT-013]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open three production-served tabs from fresh documents; use the same session in two and keep another on the desktop."
      - "Send a useful first prompt through the visible composer and confirm the durable transcript independently."
      - "Stop a live session, verify the second tab converges, refresh, and confirm history and active-catalog removal through HTTP and UDS."
      - "Change a profile in another public surface while its browser tab is hidden, then confirm the browser receives the lifecycle update; reconnect after a network interruption."
    must_avoid:
      - "Do not equate this transport replay with the separate startup latency or full profile identity picker charter."
```

<!-- Reuse this mission; append each execution debrief to its dated report. -->
