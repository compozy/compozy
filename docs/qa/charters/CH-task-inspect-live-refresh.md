# CH-task-inspect-live-refresh: Task detail and Inspect share one live owner

```yaml
charter:
  id: CH-task-inspect-live-refresh
  mission: "As Bruno, inspect a live task through one detail window and prove Inspect shares its stream, coalesces event bursts, and stays truthful through close, reopen, hide, and restore."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [TA-task-inspect-single-live-stream]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open a live task and Inspect, then count task SSE connections and one visible update for one server event."
      - "Generate a short event burst and confirm the final detail/Inspect state without repeated rows or refresh flicker."
      - "Close and reopen Inspect, then hide and restore the task window; no extra connection may survive and catch-up must not require reload."
    must_avoid:
      - "Do not infer stream ownership from source or unit tests; observe the public Web surface and browser Network traffic."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
