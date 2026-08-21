# CH-run-agent-terminal-cleanup: Settle run-agent sessions on every terminal path

```yaml
charter:
  id: CH-run-agent-terminal-cleanup
  mission: "As Bruno, drive one run-agent cell through retry, final failure, and cancellation, and trust that only reusable work keeps its worker session active."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-run-agent-session-lifecycle, LP-run-agent-output-ownership]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start each Loop through the structured CLI and capture the exact run-agent worker session."
      - "Allow a retry-eligible failure to advance to its next attempt before checking terminal cleanup."
      - "Reach one final failure and cancel one live node lane, then confirm each worker is stopped through a fresh CLI read and an independent HTTP read."
      - "Confirm schema-valid output still settles through the daemon-owned task path."
    must_avoid:
      - "Using database reads, unit tests, or source inspection as pass evidence."
      - "Stopping a worker manually before its terminal-state check."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
